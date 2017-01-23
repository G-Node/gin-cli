package repo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/G-Node/gin-cli/auth"
	"github.com/G-Node/gin-cli/util"
	git "gopkg.in/libgit2/git2go.v24"
)

// Keys

// Temporary (SSH key) file handling

var privKeyFile util.TempFile

// MakeTempKeyPair creates a temporary key pair and stores it in a temporary directory.
// It also sets the global tempFile for use by the annex commands. The key pair is returned directly.
func (repocl *Client) MakeTempKeyPair() (*util.KeyPair, error) {
	tempKeyPair, err := util.MakeKeyPair()
	if err != nil {
		return nil, err
	}

	description := fmt.Sprintf("tmpkey@%s", strconv.FormatInt(time.Now().Unix(), 10))
	pubkey := fmt.Sprintf("%s %s", strings.TrimSpace(tempKeyPair.Public), description)
	authcl := auth.NewClient(repocl.KeyHost)
	err = authcl.AddKey(pubkey, description, true)
	if err != nil {
		return tempKeyPair, err
	}

	privKeyFile, err = util.MakeTempFile("priv")
	if err != nil {
		return tempKeyPair, err
	}

	err = privKeyFile.Write(tempKeyPair.Private)
	if err != nil {
		return tempKeyPair, err
	}

	privKeyFile.Active = true

	return tempKeyPair, nil
}

// CleanUpTemp deletes the temporary directory which holds the temporary private key if it exists.
func CleanUpTemp() {
	if privKeyFile.Active {
		privKeyFile.Delete()
	}
}

// Git callbacks

func (repocl *Client) makeCredsCB() git.CredentialsCallback {
	// attemptnum is used to determine which authentication method to use each time.
	attemptnum := 0

	return func(url string, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
		var res int
		var cred git.Cred
		switch attemptnum {
		case 0:
			res, cred = git.NewCredSshKeyFromAgent(repocl.GitUser)
		case 1:
			tempKeyPair, err := repocl.MakeTempKeyPair()
			if err != nil {
				return git.ErrUser, nil
			}
			res, cred = git.NewCredSshKeyFromMemory(repocl.GitUser, tempKeyPair.Public, tempKeyPair.Private, "")
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

func (repocl *Client) certCB(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	// TODO: Better cert check?
	if hostname != repocl.GitHost {
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
	println("Adding files")
	repo, err := getRepo(localPath)
	if err != nil {
		return nil, err
	}
	idx, err := repo.Index()
	if err != nil {
		return nil, err
	}
	err = AnnexAdd(localPath, idx)
	var i uint
	println("Adding paths")
	for i = 0; i < idx.EntryCount(); i++ {
		ie, _ := idx.EntryByIndex(i)
		println(i, ie.Path)
	}
	return idx, err
}

// Connect opens a connection to the git server. This is used to validate credentials
// and generate temporary keys on demand, without performing a git operation.
func (repocl *Client) Connect(localPath string, push bool) error {
	var dir git.ConnectDirection
	if push {
		dir = git.ConnectDirectionPush
	} else {
		dir = git.ConnectDirectionFetch
	}

	cbs := &git.RemoteCallbacks{
		CredentialsCallback:      repocl.makeCredsCB(),
		CertificateCheckCallback: repocl.certCB,
	}

	var headers []string

	repository, err := git.OpenRepository(localPath)
	if err != nil {
		return err
	}

	origin, err := repository.Remotes.Lookup("origin")
	if err != nil {
		return err
	}

	return origin.Connect(dir, cbs, headers)
}

// Clone downloads a repository and sets the remote fetch and push urls.
// (git clone ...)
func (repocl *Client) Clone(repopath string) (*git.Repository, error) {
	remotePath := fmt.Sprintf("%s@%s:%s", repocl.GitUser, repocl.GitHost, repopath)
	localPath := path.Base(repopath)

	cbs := &git.RemoteCallbacks{
		CredentialsCallback:      repocl.makeCredsCB(),
		CertificateCheckCallback: repocl.certCB,
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
func (repocl *Client) Commit(localPath string, idx *git.Index) error {
	// TODO: Construct signature based on user config
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
func (repocl *Client) Pull(localPath string) error {
	repository, err := git.OpenRepository(localPath)
	if err != nil {
		return err
	}

	origin, err := repository.Remotes.Lookup("origin")
	if err != nil {
		return err
	}

	// Fetch
	cbs := &git.RemoteCallbacks{
		CredentialsCallback:      repocl.makeCredsCB(),
		CertificateCheckCallback: repocl.certCB,
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
func (repocl *Client) Push(localPath string) error {
	repository, err := git.OpenRepository(localPath)
	if err != nil {
		return err
	}

	origin, err := repository.Remotes.Lookup("origin")
	if err != nil {
		return err
	}

	rcbs := git.RemoteCallbacks{
		CredentialsCallback:      repocl.makeCredsCB(),
		CertificateCheckCallback: repocl.certCB,
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
	initError := fmt.Errorf("Repository annex initialisation failed.")
	cmd := exec.Command("git", "-C", localPath, "annex", "init", "--version=6")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.SSHOptString())
	}
	err := cmd.Run()
	if err != nil {
		return initError
	}

	err = exec.Command("git", "-C", localPath, "config", "annex.addunlocked", "true").Run()
	if err != nil {
		return initError
	}

	// list of extensions that are added to git (not annex)
	// TODO: Read from file
	gitexts := [...]string{"md", "rst", "txt", "c", "cpp", "h", "hpp", "py", "go"}
	includes := make([]string, len(gitexts))
	for idx, ext := range gitexts {
		includes[idx] = fmt.Sprintf("include=*.%s", ext)
	}
	sizethreshold := "10M"
	lfvalue := fmt.Sprintf("largerthan=%s and not (%s)", sizethreshold, strings.Join(includes, " or "))
	err = exec.Command("git", "-C", localPath, "config", "annex.largefiles", lfvalue).Run()
	if err != nil {
		return initError
	}
	err = exec.Command("git", "-C", localPath, "config", "annex.backends", "WORM").Run()
	if err != nil {
		return initError
	}
	err = exec.Command("git", "-C", localPath, "config", "annex.thin", "true").Run()
	if err != nil {
		return initError
	}
	return nil
}

// AnnexPull downloads all annexed files.
// (git annex sync --no-push --content)
// (git annex get --all)
func AnnexPull(localPath string) error {
	cmd := exec.Command("git", "-C", localPath, "annex", "get", "--all")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.SSHOptString())
	}
	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("Error downloading files: %s", err.Error())
	}
	return nil
}

// AnnexSync synchronises the local repository with the remote.
// (git annex sync --content)
func AnnexSync(localPath string) error {
	cmd := exec.Command("git", "-C", localPath, "annex", "sync", "--content")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.SSHOptString())
	}
	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("Error synchronising files: %s", err.Error())
	}
	return nil
}

// AnnexPush uploads all annexed files.
// (git annex sync --no-pull --content)
func AnnexPush(localPath string) error {
	cmd := exec.Command("git", "-C", localPath, "annex", "sync", "--no-pull", "--content")
	println("Performing push")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.SSHOptString())
	}
	println("CMD: ", strings.Join(cmd.Args, " "))
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
	}

	return nil
}
