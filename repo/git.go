package repo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/G-Node/gin-cli/auth"
	"github.com/G-Node/gin-cli/util"
	git "github.com/libgit2/git2go"
)

// TODO: Load from config
const user = "git"
const githost = "gin.g-node.org"

// Keys
type tempFile struct {
	Dir      string
	Filename string
	Active   bool
}

var privKeyFile tempFile

func makeTempFile(filename string) tempFile {
	dir, err := ioutil.TempDir("", "gin")
	util.CheckErrorMsg(err, fmt.Sprintf("Error creating temporary key directory."))
	newfile := tempFile{
		Dir:      dir,
		Filename: filename,
		Active:   true,
	}
	return newfile
}

func (tf tempFile) Write(content string) {
	err := ioutil.WriteFile(tf.FullPath(), []byte(content), 0600)
	util.CheckErrorMsg(err, "Error writing temporary file.")
}

func (tf tempFile) Delete() {
	_ = os.RemoveAll(tf.Dir)
}

func (tf tempFile) FullPath() string {
	return filepath.Join(tf.Dir, tf.Filename)
}

func (tf tempFile) SSHOptString() string {
	return fmt.Sprintf("annex.ssh-options=-i %s", tf.FullPath())
}

// CleanUpTemp deletes the temporary directory which holds the temporary private key if it exists.
func CleanUpTemp() {
	privKeyFile.Delete()
}

// Git callbacks

func makeCredsCB() git.CredentialsCallback {
	// attemptnum is used to determine which authentication method to use each time.
	attemptnum := 0

	return func(url string, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
		var res int
		var cred git.Cred
		switch attemptnum {
		case 0:
			res, cred = git.NewCredSshKeyFromAgent("git")
		case 1:
			tempKeyPair := util.MakeKeyPair()
			description := fmt.Sprintf("tmpkey@%s", strconv.FormatInt(time.Now().Unix(), 10))
			pubkey := fmt.Sprintf("%s %s", strings.TrimSpace(tempKeyPair.Public), description)
			err := auth.AddKey(pubkey, description)
			util.CheckError(err)
			privKeyFile = makeTempFile("priv")
			privKeyFile.Write(tempKeyPair.Private)
			util.CheckError(err)
			res, cred = git.NewCredSshKeyFromMemory("git", tempKeyPair.Public, tempKeyPair.Private, "")
		default:
			return git.ErrUser, nil
		}

		if res != 0 {
			return git.ErrorCode(res), nil
		}
		attemptnum++
		return git.ErrOk, &cred
	}
}

func certCB(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	// TODO: Better cert check?
	if hostname != githost {
		return git.ErrCertificate
	}
	return git.ErrOk
}

func remoteCreateCB(repo *git.Repository, name, url string) (*git.Remote, git.ErrorCode) {
	remote, err := repo.Remotes.Create(name, url)
	if err != nil {
		return nil, git.ErrUser
	}
	return remote, git.ErrOk
}

func matchPathCB(p, mp string) int {
	return 0
}

// **************** //

// Git commands

// IsRepo checks whether a given path is a git repository.
func IsRepo(path string) bool {
	_, err := git.Discover(path, false, nil)
	if err != nil {
		return false
	}
	return true
}

func getRepo(startPath string) *git.Repository {
	localRepoPath, err := git.Discover(startPath, false, nil)
	util.CheckError(err)
	repo, err := git.OpenRepository(localRepoPath)
	util.CheckError(err)
	return repo
}

// AddPath adds files or directories to the index
func AddPath(localPath string) *git.Index {
	repo := getRepo(localPath)
	idx, err := repo.Index()
	util.CheckError(err)
	// Currently adding everything to annex
	// Eventually will decide on what is versioned and what is annexed based on MIME type
	// err = idx.AddAll([]string{localPath}, git.IndexAddDefault, matchPathCB)
	// if err != nil {
	// 	return err
	// }
	// return idx.Write()

	AnnexAdd(localPath, idx)
	return idx
}

// Clone downloads a repository and sets the remote fetch and push urls.
// (git clone ...)
func Clone(repopath string) *git.Repository {
	remotePath := fmt.Sprintf("%s@%s:%s", user, githost, repopath)
	localPath := path.Base(repopath)

	cbs := &git.RemoteCallbacks{
		CredentialsCallback:      makeCredsCB(),
		CertificateCheckCallback: certCB,
	}
	fetchopts := &git.FetchOptions{RemoteCallbacks: *cbs}
	opts := git.CloneOptions{
		Bare:                 false,
		CheckoutBranch:       "master", // TODO: default branch
		FetchOptions:         fetchopts,
		RemoteCreateCallback: remoteCreateCB,
	}
	repository, err := git.Clone(remotePath, localPath, &opts)
	util.CheckError(err)
	return repository
}

// Commit performs a git commit on the currently staged objects.
// (git commit)
func Commit(localPath string, idx *git.Index) {
	signature := &git.Signature{
		Name:  "gin",
		Email: "gin",
		When:  time.Now(),
	}
	repository, err := git.OpenRepository(localPath)
	util.CheckError(err)
	head, err := repository.Head()
	util.CheckError(err)
	headCommit, err := repository.LookupCommit(head.Target())
	util.CheckError(err)

	message := "uploading" // TODO: Describe changes (in message)
	treeID, err := idx.WriteTree()
	util.CheckError(err)
	err = idx.Write()
	util.CheckError(err)
	tree, err := repository.LookupTree(treeID)
	util.CheckError(err)
	_, err = repository.CreateCommit("HEAD", signature, signature, message, tree, headCommit)
	util.CheckError(err)

}

// Pull pulls all remote commits from the default remote & branch
// (git pull)
func Pull() {
	repository, err := git.OpenRepository(".")
	util.CheckError(err)

	origin, err := repository.Remotes.Lookup("origin")
	util.CheckError(err)

	// Fetch
	cbs := &git.RemoteCallbacks{
		CredentialsCallback:      makeCredsCB(),
		CertificateCheckCallback: certCB,
	}
	fetchopts := &git.FetchOptions{RemoteCallbacks: *cbs}

	err = origin.Fetch([]string{}, fetchopts, "")
	util.CheckError(err)

	// Merge
	remoteBranch, err := repository.References.Lookup("refs/remotes/origin/master")
	util.CheckError(err)

	annotatedCommit, err := repository.AnnotatedCommitFromRef(remoteBranch)
	util.CheckError(err)

	mergeHeads := make([]*git.AnnotatedCommit, 1)
	mergeHeads[0] = annotatedCommit
	analysis, _, err := repository.MergeAnalysis(mergeHeads)
	util.CheckError(err)

	if analysis&git.MergeAnalysisUpToDate != 0 {
		// Nothing to do
		return
	} else if analysis&git.MergeAnalysisNormal != 0 {
		// Merge changes
		err := repository.Merge([]*git.AnnotatedCommit{annotatedCommit}, nil, nil)
		util.CheckError(err)

		// Check for conflicts
		index, err := repository.Index()
		util.CheckError(err)

		if index.HasConflicts() {
			util.CheckError(fmt.Errorf("Merge conflicts encountered.")) // TODO: Automatically resolve?
		}

		// Create merge commit
		signature, err := repository.DefaultSignature() // TODO: Signature should use username and email if public on gin-auth
		util.CheckError(err)

		treeID, err := index.WriteTree()
		util.CheckError(err)

		tree, err := repository.LookupTree(treeID)
		util.CheckError(err)

		head, err := repository.Head()
		util.CheckError(err)

		localCommit, err := repository.LookupCommit(head.Target())
		util.CheckError(err)

		remoteCommit, err := repository.LookupCommit(remoteBranch.Target())
		util.CheckError(err)

		_, err = repository.CreateCommit("HEAD", signature, signature, "", tree, localCommit, remoteCommit)
		util.CheckError(err)

		err = repository.StateCleanup()
		util.CheckError(err)

	}
}

// Push pushes all local commits to the default remote & branch
// (git push)
func Push(localPath string) {
	repository, err := git.OpenRepository(localPath)
	util.CheckError(err)

	origin, err := repository.Remotes.Lookup("origin")
	util.CheckError(err)

	rcbs := git.RemoteCallbacks{
		CredentialsCallback:      makeCredsCB(),
		CertificateCheckCallback: certCB,
	}

	popts := &git.PushOptions{
		RemoteCallbacks: rcbs,
	}
	refspecs := []string{"refs/heads/master"}
	err = origin.Push(refspecs, popts)
	util.CheckError(err)
}

// **************** //

// Git annex commands

// AnnexPull downloads all annexed files.
// (git annex sync --no-push --content)
func AnnexPull(localPath string) {
	cmd := exec.Command("git", "-C", localPath, "annex", "sync", "--no-push", "--content")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.SSHOptString())
	}
	err := cmd.Run()
	util.CheckErrorMsg(err, "Error downloading files.")
}

// AnnexPush uploads all annexed files.
// (git annex sync --no-pull --content)
func AnnexPush(localPath string) {
	cmd := exec.Command("git", "-C", localPath, "annex", "sync", "--no-pull", "--content")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.SSHOptString())
	}
	err := cmd.Run()
	util.CheckErrorMsg(err, "Error uploading files.")
}

// AnnexAddResult ...
type AnnexAddResult struct {
	Command string `json:"command"`
	File    string `json:"file"`
	Key     string `json:"key"`
	Success bool   `json:"success"`
}

// AnnexAdd adds a path to the annex.
// (git annex add)
func AnnexAdd(localPath string, idx *git.Index) {
	// TODO: Return error if no new files are added
	out, err := exec.Command("git", "annex", "--json", "add", localPath).Output()

	util.CheckErrorMsg(err, "Error adding files to repository.")

	var outStruct AnnexAddResult
	files := bytes.Split(out, []byte("\n"))
	for _, f := range files {
		if len(f) == 0 {
			break
		}
		err := json.Unmarshal(f, &outStruct)
		util.CheckError(err)
		if !outStruct.Success {
			util.Die(fmt.Sprintf("Error adding files to repository: Failed to add %s", outStruct.File))
		}
		err = idx.AddByPath(outStruct.File)
		util.CheckError(err)
	}
}
