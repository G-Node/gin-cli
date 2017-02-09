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
func AddPath(localPath string) ([]string, error) {
	return AnnexAdd(localPath)
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

	repository, err := getRepo(localPath)
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
	repository, err := getRepo(localPath)
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

	message := "uploading"
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
	repository, err := getRepo(localPath)
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
	repository, err := getRepo(localPath)
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
	// cmd := exec.Command("git", "-C", localPath, "annex", "get", "--all")
	cmd := exec.Command("git", "-C", localPath, "annex", "sync", "--no-push", "--content")
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.SSHOptString())
	}
	out, err := cmd.Output()

	if err != nil {
		return fmt.Errorf("Error downloading files: %s", out)
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
func AnnexPush(localPath, commitMsg string) error {
	cmd := exec.Command("git", "-C", localPath, "annex", "sync", "--no-pull", "--content", "--commit", fmt.Sprintf("--message=%s", commitMsg))
	if privKeyFile.Active {
		cmd.Args = append(cmd.Args, "-c", privKeyFile.SSHOptString())
	}
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		return fmt.Errorf("Error uploading files: %s", stderr.String())
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
func AnnexAdd(localPath string) ([]string, error) {
	cmd := exec.Command("git", "annex", "--json", "add", localPath)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		return nil, fmt.Errorf("Error adding files to repository: %s", stderr.String())
	}

	var outStruct AnnexAddResult
	files := bytes.Split(out.Bytes(), []byte("\n"))
	added := make([]string, 0, len(files))
	for _, f := range files {
		if len(f) == 0 {
			continue
		}
		err := json.Unmarshal(f, &outStruct)
		if err != nil {
			return nil, err
		}
		if !outStruct.Success {
			return nil, fmt.Errorf("Error adding files to repository: Failed to add %s", outStruct.File)
		}
		added = append(added, outStruct.File)
	}

	return added, nil
}

func repoIndexPaths(localPath string) ([]string, error) {
	repo, err := getRepo(localPath)
	if err != nil {
		return nil, err
	}

	index, err := repo.Index()
	if err != nil {
		return nil, err
	}

	entries := make([]string, index.EntryCount())
	for idx := uint(0); idx < index.EntryCount(); idx++ {
		entry, err := index.EntryByIndex(idx)
		if err != nil {
			return nil, err
		}
		entries[idx] = entry.Path
	}

	return entries, nil
}

// AnnexStatusResult ...
type AnnexStatusResult struct {
	Status string `json:"status"`
	File   string `json:"file"`
}

// DescribeChanges returns a string which describes the status of the files in the working tree
// with respect to git annex. The resulting message can be used to inform the user of changes
// that are about to be uploaded and as a long commit message.
func DescribeChanges(localPath string) (string, error) {
	cmd := exec.Command("git", "-C", localPath, "annex", "status", "--json")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		return "", fmt.Errorf("Error retrieving file status: %s", stderr.String())
	}

	var outStruct AnnexStatusResult
	files := bytes.Split(out.Bytes(), []byte("\n"))

	statusmap := make(map[string][]string)
	for _, f := range files {
		if len(f) == 0 {
			continue
		}
		err := json.Unmarshal(f, &outStruct)
		if err != nil {
			return "", err
		}
		statusmap[outStruct.Status] = append(statusmap[outStruct.Status], outStruct.File)
	}

	var changeList string
	changeList += makeFileList("New files", statusmap["A"])
	changeList += makeFileList("Modified files", statusmap["M"])
	changeList += makeFileList("Deleted files", statusmap["D"])
	changeList += makeFileList("Type modified files", statusmap["T"])
	changeList += makeFileList("Untracked files ", statusmap["?"])

	return changeList, nil
}

func makeFileList(header string, fnames []string) (list string) {
	if len(fnames) == 0 {
		return
	}
	list += fmt.Sprint(header) + "\n"
	for idx, name := range fnames {
		list += fmt.Sprintf("  %d: %s\n", idx, name)
	}
	list += "\n"
	return
}
