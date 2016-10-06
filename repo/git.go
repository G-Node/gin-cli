package repo

import (
	"fmt"
	"os/exec"
	"path"

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

func getRepo(startPath string) (*git.Repository, error) {
	localRepoPath, err := git.Discover(startPath, false, nil)
	if err != nil {
		return nil, err
	}
	return git.OpenRepository(localRepoPath)
}

// AddPath adds files or directories to the index
func AddPath(localPath string) error {
	// repo, err := getRepo(localPath)
	// if err != nil {
	// 	return err
	// }
	// idx, err := repo.Index()
	// if err != nil {
	// 	return err
	// }
	// Currently adding everything to annex
	// Eventually will decide on what is versioned and what is annexed based on MIME type
	// err = idx.AddAll([]string{localPath}, git.IndexAddDefault, matchPathCB)
	// if err != nil {
	// 	return err
	// }
	// return idx.Write()

	return AnnexAdd(localPath)
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
		CheckoutBranch:       "master",
		FetchOptions:         fetchopts,
		RemoteCreateCallback: remoteCreateCB,
	}
	repository, err := git.Clone(remotePath, localPath, &opts)

	if err != nil {
		return nil, err
	}

	return repository, nil
}

// AnnexPull downloads all annexed files.
// (git annex sync --no-push --content)
func AnnexPull(localPath string) error {
	_, err := exec.Command("git", "-C", localPath, "annex", "sync", "--no-push", "--content").Output()

	if err != nil {
		return fmt.Errorf("Error downloading files: %s", err.Error())
	}
	return nil
}

// AnnexAdd adds a path to the annex
// (git annex add)
func AnnexAdd(localPath string) error {
	_, err := exec.Command("git", "annex", "add", localPath).Output()

	if err != nil {
		return fmt.Errorf("Error adding files to repository: %s", err.Error())
	}
	return nil
}
