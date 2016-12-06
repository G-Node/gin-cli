package repo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/G-Node/gin-cli/web"
	"github.com/G-Node/gin-repo/wire"
)

const repohost = "https://repo.gin.g-node.org"

// Client is a client interface to the repo server. Embeds web.Client.
type Client struct {
	*web.Client
}

// NewClient returns a new client for the auth server.
func NewClient() *Client {
	serverURL := repohost
	return &Client{web.NewClient(serverURL)}
}

// GetRepos gets a list of repositories (public or user specific)
func (repocl *Client) GetRepos(user, token string) ([]wire.Repo, error) {
	var repoList []wire.Repo
	var res *http.Response
	var err error

	if user == "" {
		res, err = repocl.Get("/repos/public")
	} else {
		repocl.Token = token
		res, err = repocl.Get(fmt.Sprintf("/users/%s/repos", user))
	}

	// util.CheckErrorMsg(err, "[Repository request] Request failed.")
	if err != nil {
		return repoList, err
	} else if res.StatusCode != 200 {
		return repoList, fmt.Errorf("[Repository request] Failed. Server returned: %s", res.Status)
	}

	defer web.CloseRes(res.Body)
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return repoList, err
	}
	err = json.Unmarshal(b, &repoList)
	return repoList, err
}

// CreateRepo creates a repository on the server.
func (repocl *Client) CreateRepo(name, description string) error {
	err := repocl.LoadToken()
	if err != nil {
		return fmt.Errorf("[Create repository] This action requires login")
	}

	data := wire.Repo{Name: name, Description: description}
	res, err := repocl.Post(fmt.Sprintf("/users/%s/repos", repocl.Username), data)
	if err != nil {
		// util.CheckErrorMsg(err, "[Create repository] Request failed.")
		return err
	} else if res.StatusCode != 201 {
		// util.Die(fmt.Sprintf("[Create repository] Failed. Server returned %s", res.Status))
		return fmt.Errorf("[Create repository] Failed. Server returned %s", res.Status)
	}
	web.CloseRes(res.Body)
	return nil
}

// UploadRepo adds files to a repository and uploads them.
func (repocl *Client) UploadRepo(localPath string) error {
	defer CleanUpTemp()

	_, err := AddPath(localPath)
	if err != nil {
		return err
	}

	// since no git command is called, we need to explicitly create temporary keys
	_, err = setupTempKeyPair()
	util.CheckError(err)

	err = AnnexPush(localPath)
	return err
}

// DownloadRepo downloads the files in an already checked out repository.
func (repocl *Client) DownloadRepo() error {
	defer CleanUpTemp()
	err := AnnexPull(".")
	return err
}

// CloneRepo downloads the files of a given repository.
func (repocl *Client) CloneRepo(repoPath string) error {
	defer CleanUpTemp()

	localPath := path.Base(repoPath)
	fmt.Printf("Fetching repository '%s'... ", localPath)
	_, err := Clone(repoPath)
	if err != nil {
		return err
	}
	fmt.Printf("done.\n")

	// git annex init the clone and set defaults
	err = AnnexInit(localPath)
	if err != nil {
		return err
	}

	// TODO: Do not try to download files if repo is empty
	fmt.Printf("Downloading files... ")
	err = AnnexPull(localPath)
	if err != nil {
		return err
	}
	fmt.Printf("done.\n")
	return nil
}
