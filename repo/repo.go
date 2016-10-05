package repo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/G-Node/gin-cli/auth"
	"github.com/G-Node/gin-cli/client"
	"github.com/G-Node/gin-repo/wire"
)

const repohost = "https://repo.gin.g-node.org"

// GetRepos gets a list of repositories (public or user specific)
func GetRepos(user, token string) ([]wire.Repo, error) {
	repocl := client.NewClient(repohost)
	var res *http.Response
	var err error
	if user == "" {
		res, err = repocl.Get("/repos/public")
	} else {
		repocl.Token = token
		res, err = repocl.Get(fmt.Sprintf("/users/%s/repos", user))
	}

	if err != nil {
		return nil, fmt.Errorf("Request for repositories returned error: %s", err)
	} else if res.StatusCode != 200 {
		return nil, fmt.Errorf("[Repository request error] Server returned: %s", res.Status)
	}

	defer client.CloseRes(res.Body)
	b, err := ioutil.ReadAll(res.Body)
	var repoList []wire.Repo
	err = json.Unmarshal(b, &repoList)
	if err != nil {
		return nil, err
	}

	return repoList, nil
}

// CreateRepo creates a repository on the server
func CreateRepo(name, description string) error {
	repocl := client.NewClient(repohost)
	username, token, _ := auth.LoadToken(false)
	repocl.Token = token

	data := wire.Repo{Name: name, Description: description}
	res, err := repocl.Post(fmt.Sprintf("/users/%s/repos", username), data)
	if err != nil {
		return err
	}
	defer client.CloseRes(res.Body)
	if res.StatusCode != 201 {
		return fmt.Errorf("Failed to create repository. Server returned: %s", res.Status)
	}

	// TODO: Initialise? Do a git annex init and push?

	return nil
}

// ResolvePath resolves a valid repository path given a user's input.
func ResolvePath(path string) (string, error) {
	// TODO: Write the function, eh?
	return path, nil
}

// UploadRepo adds files to a repository and upload.
func UploadRepo(path string) error {
	return nil
}

// DownloadRepo downloads the files of a given repository.
func DownloadRepo(repopath string) error {
	_, err := Clone(repopath)
	if err != nil {
		return err
	}
	return nil
}
