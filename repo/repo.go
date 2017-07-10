package repo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/gogits/go-gogs-client"
	// its a bit unfortunate that we have that import now
	// but its only temporary...
	"github.com/G-Node/gin-cli/auth"
)

// Client is a client interface to the repo server. Embeds web.Client.
type Client struct {
	*web.Client
	KeyHost string
	GitHost string
	GitUser string
}

// NewClient returns a new client for the repo server.
func NewClient(host string) *Client {
	return &Client{Client: web.NewClient(host)}
}

// GetRepo retrieves the information of a repository.
func (repocl *Client) GetRepo(repoPath string) (gogs.Repository, error) {
	defer CleanUpTemp()
	util.LogWrite("GetRepo")
	var repo gogs.Repository

	res, err := repocl.Get(fmt.Sprintf("/api/v1/repos/%s", repoPath))
	if err != nil {
		return repo, err
	} else if res.StatusCode == http.StatusNotFound {
		return repo, fmt.Errorf("Not found. Check repository owner and name.")
	} else if res.StatusCode == http.StatusUnauthorized {
		return repo, fmt.Errorf("You are not authorised to access repository.")
	} else if res.StatusCode != http.StatusOK {
		return repo, fmt.Errorf("Server returned %s", res.Body)
	}
	defer web.CloseRes(res.Body)
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return repo, err
	}
	err = json.Unmarshal(b, &repo)
	return repo, err
}

// GetRepos gets a list of repositories (public or user specific)
func (repocl *Client) GetRepos(user string) ([]gogs.Repository, error) {
	util.LogWrite("Retrieving repo list")
	var repoList []gogs.Repository
	var res *http.Response
	var err error
	res, err = repocl.Get("/api/v1/user/repos")
	if err != nil {
		return repoList, err
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
	util.LogWrite("Creating repository")
	err := repocl.LoadToken()
	if err != nil {
		return fmt.Errorf("[Create repository] This action requires login")
	}

	newrepo := gogs.Repository{Name: name, Description: description}
	util.LogWrite("Name: %s :: Description: %s", name, description)
	res, err := repocl.Post("/api/v1/user/repos", newrepo)
	if err != nil {
		return err
	} else if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("[Create repository] Failed. Server returned %s", res.Status)
	}
	web.CloseRes(res.Body)
	util.LogWrite("Repository created")
	return nil
}

// DelRepo deletes a repository from the server.
func (repocl *Client) DelRepo(name string) error {
	util.LogWrite("Deleting repository")
	err := repocl.LoadToken()
	if err != nil {
		return fmt.Errorf("[Delete repository] This action requires login")
	}

	res, err := repocl.Delete(fmt.Sprintf("/api/v1/repos/%s", name))
	if err != nil {
		return err
	} else if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("[Delete repository] Failed. Server returned %s", res.Status)
	}
	web.CloseRes(res.Body)
	util.LogWrite("Repository deleted")
	return nil
}

// UploadRepo adds files to a repository and uploads them.
func (repocl *Client) UploadRepo(localPath string) error {
	defer auth.NewClient(repocl.Host).DeleteTmpKeys()
	defer CleanUpTemp()
	util.LogWrite("UploadRepo")

	err := repocl.Connect()
	if err != nil {
		return err
	}

	added, err := AnnexAdd(localPath)
	if err != nil {
		return err
	}

	if len(added) == 0 {
		return fmt.Errorf("No changes to upload")
	}

	changes, err := DescribeChanges(localPath)
	// add header commit line
	changes = fmt.Sprintf("gin upload\n\n%s", changes)
	if err != nil {
		return err
	}

	// fmt.Println(changes)

	err = AnnexPush(localPath, changes)
	return err
}

// DownloadRepo downloads the files in an already checked out repository.
func (repocl *Client) DownloadRepo(localPath string) error {
	defer auth.NewClient(repocl.Host).DeleteTmpKeys()
	defer CleanUpTemp()
	util.LogWrite("DownloadRepo")

	err := repocl.Connect()
	if err != nil {
		return err
	}
	err = AnnexPull(localPath)
	return err
}

// GetContent retrieves the contents of placeholder files in a checked out repository.
func (repocl *Client) GetContent(localPath string, filepaths []string) error {
	defer CleanUpTemp()
	util.LogWrite("GetContent")

	err := repocl.Connect()
	if err != nil {
		return err
	}
	err = AnnexGet(localPath, filepaths)
	return err
}

// CloneRepo clones a remote repository and initialises anex init with the options specified in the config file.
func (repocl *Client) CloneRepo(repoPath string) error {
	defer auth.NewClient(repocl.Host).DeleteTmpKeys()
	defer CleanUpTemp()
	util.LogWrite("CloneRepo")

	err := repocl.Connect()
	if err != nil {
		return err
	}

	_, repoName := splitRepoParts(repoPath)
	fmt.Printf("Fetching repository '%s'... ", repoPath)
	err = repocl.Clone(repoPath)
	if err != nil {
		return err
	}
	fmt.Printf("done.\n")

	// git annex init the clone and set defaults
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	err = repocl.LoadToken()
	if err != nil {
		return err
	}
	description := fmt.Sprintf("%s@%s", repocl.Username, hostname)
	return AnnexInit(repoName, description)
}
