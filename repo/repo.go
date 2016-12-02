package repo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/G-Node/gin-cli/auth"
	"github.com/G-Node/gin-cli/client"
	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-repo/wire"
)

const repohost = "https://repo.gin.g-node.org"

// GetRepos gets a list of repositories (public or user specific)
func GetRepos(user, token string) []wire.Repo {
	repocl := client.NewClient(repohost)
	var res *http.Response
	var err error
	if user == "" {
		res, err = repocl.Get("/repos/public")
	} else {
		repocl.Token = token
		res, err = repocl.Get(fmt.Sprintf("/users/%s/repos", user))
	}

	util.CheckErrorMsg(err, "[Repository request] Request failed.")
	if res.StatusCode != 200 {
		util.Die(fmt.Sprintf("[Repository request] Failed. Server returned: %s", res.Status))
	}

	defer client.CloseRes(res.Body)
	b, err := ioutil.ReadAll(res.Body)
	var repoList []wire.Repo
	err = json.Unmarshal(b, &repoList)
	util.CheckError(err)
	return repoList
}

// CreateRepo creates a repository on the server.
func CreateRepo(name, description string) {
	repocl := client.NewClient(repohost)
	username, token, err := auth.LoadToken(false)
	util.CheckErrorMsg(err, "[Create repository] This action requires login.")
	repocl.Token = token

	data := wire.Repo{Name: name, Description: description}
	res, err := repocl.Post(fmt.Sprintf("/users/%s/repos", username), data)
	util.CheckErrorMsg(err, "[Create repository] Request failed.")
	if res.StatusCode != 201 {
		util.Die(fmt.Sprintf("[Create repository] Failed. Server returned %s", res.Status))
	}
	client.CloseRes(res.Body)
}

// UploadRepo adds files to a repository and uploads them.
func UploadRepo(localPath string) {
	defer CleanUpTemp()

	_, err := AddPath(localPath)
	util.CheckError(err)

	// since no git command is called, we need to explicitly create temporary keys
	_, err = MakeTempKeyPair()
	util.CheckError(err)

	err = AnnexPush(localPath)
	util.CheckError(err)
}

// DownloadRepo downloads the files in an already checked out repository.
func DownloadRepo() {
	defer CleanUpTemp()
	err := AnnexPull(".")
	util.CheckError(err)
}

// CloneRepo downloads the files of a given repository.
func CloneRepo(repoPath string) {
	defer CleanUpTemp()

	localPath := path.Base(repoPath)
	fmt.Printf("Fetching repository '%s'... ", localPath)
	_, err := Clone(repoPath)
	util.CheckError(err)
	fmt.Printf("done.\n")

	// git annex init the clone and set defaults
	err = AnnexInit(localPath)
	util.CheckError(err)

	// TODO: Do not try to download files if repo is empty
	fmt.Printf("Downloading files... ")
	err = AnnexPull(localPath)
	util.CheckError(err)
	fmt.Printf("done.\n")
}
