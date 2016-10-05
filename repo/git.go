package repo

import (
	"fmt"
	"path"

	git "github.com/libgit2/git2go"
)

const user = "git"
const githost = "gin.g-node.org"

func credsCB(url string, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
	println("Credentials callback")
	_, cred := git.NewCredSshKeyFromAgent("git")
	return 0, &cred
}

func certCB(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	println("Certificate callback")
	// if hostname != "gin.g-node.org" { // TODO: Read from config
	// 	return git.ErrCertificate
	// }
	return 0
}

func remoteCreateCB(repo *git.Repository, name, url string) (*git.Remote, git.ErrorCode) {
	fmt.Printf("Creating remote [%s] with [%s]\n", name, url)
	remote, err := repo.Remotes.Create(name, url)
	if err != nil {
		return nil, 1 // TODO: Return proper error codes
	}
	return remote, 0
}

// Clone downloads a repository and sets the remote fetch and push urls
func Clone(repopath string) error {
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
	fmt.Printf("Cloning [%s] into [%s] ...\n", remotePath, localPath)
	repository, err := git.Clone(remotePath, localPath, &opts)

	if err != nil {
		println("Clone failed")
		println("Error:", err.Error())
		return err
	}

	println(repository.Path())

	return nil
}
