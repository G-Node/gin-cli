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

func makeTempFile(filename string) (tempFile, error) {
	dir, err := ioutil.TempDir("", "gin")
	if err != nil {
		return tempFile{}, fmt.Errorf("Error creating temporary key directory: %s", err)
	}
	newfile := tempFile{
		Dir:      dir,
		Filename: filename,
		Active:   true,
	}
	return newfile, nil
}

func (tf tempFile) Write(content string) error {
	if err := ioutil.WriteFile(tf.FullPath(), []byte(content), 0600); err != nil {
		return fmt.Errorf("Error writing temporary file: %s", err)
	}
	return nil
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
			tempKeyPair, err := util.MakeKeyPair()
			if err != nil {
				return git.ErrUser, nil
			}
			description := fmt.Sprintf("tmpkey@%s", strconv.FormatInt(time.Now().Unix(), 10))
			pubkey := fmt.Sprintf("%s %s", strings.TrimSpace(tempKeyPair.Public), description)
			err = auth.AddKey(pubkey, description)
			if err != nil {
				return git.ErrUser, nil
			}
			privKeyFile, err = makeTempFile("priv")
			if err != nil {
				return git.ErrUser, nil
			}
			err = privKeyFile.Write(tempKeyPair.Private)
			if err != nil {
				return git.ErrUser, nil
			}
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

func getRepo(startPath string) (*git.Repository, error) {
	localRepoPath, err := git.Discover(startPath, false, nil)
	if err != nil {
		return nil, err
	}
	return git.OpenRepository(localRepoPath)
}

// AddPath adds files or directories to the index
func AddPath(localPath string) (*git.Index, error) {
	repo, err := getRepo(localPath)
	if err != nil {
		return nil, err
	}
	idx, err := repo.Index()
	if err != nil {
		return nil, err
	}
	// Currently adding everything to annex
	// Eventually will decide on what is versioned and what is annexed based on MIME type
	// err = idx.AddAll([]string{localPath}, git.IndexAddDefault, matchPathCB)
	// if err != nil {
	// 	return err
	// }
	// return idx.Write()

	err = AnnexAdd(localPath, idx)
	return idx, err
}

// Clone downloads a repository and sets the remote fetch and push urls.
// (git clone ...)
func Clone(repopath string) (*git.Repository, error) {
	remotePath := fmt.Sprintf("%s@%s:%s", user, githost, repopath)
	localPath := path.Base(repopath)

	cbs := &git.RemoteCallbacks{
		CredentialsCallback:      makeCredsCB(),
		CertificateCheckCallback: certCB,
	}
	fetchopts := &git.FetchOptions{RemoteCallbacks: *cbs}
	opts := git.CloneOptions{
		Bare: false,
		// CheckoutBranch:       "master", // TODO: default branch
		FetchOptions:         fetchopts,
		RemoteCreateCallback: remoteCreateCB,
	}
	repository, err := git.Clone(remotePath, localPath, &opts)

	if err != nil {
		return nil, err
	}

	return repository, nil
}

// Commit performs a git commit on the currently staged objects.
// (git commit)
func Commit(localPath string, idx *git.Index) error {
	signature := &git.Signature{
		Name:  "gin",
		Email: "gin",
		When:  time.Now(),
	}
	repository, err := git.OpenRepository(localPath)
	if err != nil {
		return err
	}
	head, err := repository.Head()
	var headCommit *git.Commit
	if err != nil {
		// Head commit not found. Root commit?
		head = nil
	} else {
		headCommit, err = repository.LookupCommit(head.Target())
		if err != nil {
			return err
		}
	}

	message := "uploading" // TODO: Describe changes (in message)
	treeID, err := idx.WriteTree()
	if err != nil {
		return err
	}
	err = idx.Write()
	if err != nil {
		return err
	}
	tree, err := repository.LookupTree(treeID)
	if err != nil {
		return err
	}
	if headCommit == nil {
		_, err = repository.CreateCommit("HEAD", signature, signature, message, tree)
	} else {
		_, err = repository.CreateCommit("HEAD", signature, signature, message, tree, headCommit)
	}
	return err
}

// Pull pulls all remote commits from the default remote & branch
// (git pull)
func Pull() error {
	repository, err := git.OpenRepository(".")
	if err != nil {
		return err
	}

	origin, err := repository.Remotes.Lookup("origin")
	if err != nil {
		return err
	}

	// Fetch
	cbs := &git.RemoteCallbacks{
		CredentialsCallback:      makeCredsCB(),
		CertificateCheckCallback: certCB,
	}
	fetchopts := &git.FetchOptions{RemoteCallbacks: *cbs}

	err = origin.Fetch([]string{}, fetchopts, "")
	if err != nil {
		return err
	}

	// Merge
	remoteBranch, err := repository.References.Lookup("refs/remotes/origin/master")
	if err != nil {
		return err
	}

	annotatedCommit, err := repository.AnnotatedCommitFromRef(remoteBranch)
	if err != nil {
		return err
	}

	mergeHeads := make([]*git.AnnotatedCommit, 1)
	mergeHeads[0] = annotatedCommit
	analysis, _, err := repository.MergeAnalysis(mergeHeads)
	if err != nil {
		return err
	}

	if analysis&git.MergeAnalysisUpToDate != 0 {
		// Nothing to do
		return nil
	} else if analysis&git.MergeAnalysisNormal != 0 {
		// Merge changes
		if err := repository.Merge([]*git.AnnotatedCommit{annotatedCommit}, nil, nil); err != nil {
			return err
		}

		// Check for conflicts
		index, err := repository.Index()
		if err != nil {
			return err
		}

		if index.HasConflicts() {
			return fmt.Errorf("Merge conflicts encountered.") // TODO: Automatically resolve?
		}

		// Create merge commit
		signature, err := repository.DefaultSignature() // TODO: Signature should use username and email if public on gin-auth
		if err != nil {
			return err
		}

		treeID, err := index.WriteTree()
		if err != nil {
			return err
		}

		tree, err := repository.LookupTree(treeID)
		if err != nil {
			return err
		}

		head, err := repository.Head()
		if err != nil {
			return err
		}

		localCommit, err := repository.LookupCommit(head.Target())
		if err != nil {
			return err
		}

		remoteCommit, err := repository.LookupCommit(remoteBranch.Target())
		if err != nil {
			return err
		}

		_, err = repository.CreateCommit("HEAD", signature, signature, "", tree, localCommit, remoteCommit)
		if err != nil {
			return err
		}

		err = repository.StateCleanup()
		if err != nil {
			return err
		}

	}

	return nil
}

// Push pushes all local commits to the default remote & branch
// (git push)
func Push(localPath string) error {
	repository, err := git.OpenRepository(localPath)
	if err != nil {
		return err
	}

	origin, err := repository.Remotes.Lookup("origin")
	if err != nil {
		return err
	}

	rcbs := git.RemoteCallbacks{
		CredentialsCallback:      makeCredsCB(),
		CertificateCheckCallback: certCB,
	}

	popts := &git.PushOptions{
		RemoteCallbacks: rcbs,
	}
	refspecs := []string{"refs/heads/master"}
	return origin.Push(refspecs, popts)
}

// **************** //

// Git annex commands

// AnnexInit initialises the repository for annex
// (git annex init)
func AnnexInit(localPath string) error {
	err := exec.Command("git", "-C", localPath, "annex", "init").Run()
	if err != nil {
		return fmt.Errorf("Repository annex initialisation failed.")
	}
	return nil
}

// AnnexPull downloads all annexed files.
// (git annex sync --no-push --content)
func AnnexPull(localPath string) error {
	cmd := exec.Command("git", "-C", localPath, "annex", "sync", "--no-push", "--content")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.SSHOptString())
	}
	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("Error downloading files: %s", err.Error())
	}
	return nil
}

// AnnexPush uploads all annexed files.
// (git annex sync --no-pull --content)
func AnnexPush(localPath string) error {
	cmd := exec.Command("git", "-C", localPath, "annex", "sync", "--no-pull", "--content")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.SSHOptString())
	}
	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("Error uploading files: %s", err.Error())
	}
	return nil
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
func AnnexAdd(localPath string, idx *git.Index) error {
	// TODO: Return error if no new files are added
	out, err := exec.Command("git", "annex", "--json", "add", localPath).Output()

	if err != nil {
		return fmt.Errorf("Error adding files to repository: %s", err.Error())
	}

	var outStruct AnnexAddResult
	files := bytes.Split(out, []byte("\n"))
	for _, f := range files {
		if len(f) == 0 {
			break
		}
		err := json.Unmarshal(f, &outStruct)
		if err != nil {
			return err
		}
		if !outStruct.Success {
			return fmt.Errorf("Error adding files to repository: Failed to add %s", outStruct.File)
		}
		err = idx.AddByPath(outStruct.File)
		if err != nil {
			return err
		}
	}

	return nil
}
