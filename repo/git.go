package repo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path"
	"time"

	git "github.com/libgit2/git2go"
)

// TODO: Load from config
const user = "git"
const githost = "gin.g-node.org"

// Git callbacks

func credsCB(url string, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
	_, cred := git.NewCredSshKeyFromAgent("git")
	return git.ErrOk, &cred
}

func certCB(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	if hostname != githost {
		return git.ErrCertificate
	}
	return git.ErrOk
}

func remoteCreateCB(repo *git.Repository, name, url string) (*git.Remote, git.ErrorCode) {
	remote, err := repo.Remotes.Create(name, url)
	if err != nil {
		return nil, 1 // TODO: Return proper error codes (git.ErrorCode)
	}
	return remote, git.ErrOk
}

func matchPathCB(p, mp string) int {
	return 0
}

// **************** //

// Git commands

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
		CredentialsCallback:      credsCB,
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
	if err != nil {
		return err
	}
	headCommit, err := repository.LookupCommit(head.Target())
	if err != nil {
		return err
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
	_, err = repository.CreateCommit("HEAD", signature, signature, message, tree, headCommit)
	return err

}

// Push pushes all local commits to theh default remote & branch
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
		CredentialsCallback:      credsCB,
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

// AnnexPull downloads all annexed files.
// (git annex sync --no-push --content)
func AnnexPull(localPath string) error {
	_, err := exec.Command("git", "-C", localPath, "annex", "sync", "--no-push", "--content").Output()

	if err != nil {
		return fmt.Errorf("Error downloading files: %s", err.Error())
	}
	return nil
}

// AnnexPush uploads all annexed files.
// (git annex sync --no-pull --content)
func AnnexPush(localPath string) error {
	_, err := exec.Command("git", "-C", localPath, "annex", "sync", "--no-pull", "--content").Output()

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
