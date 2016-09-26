package repo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/G-Node/gin-cli/client"
	"github.com/G-Node/gin-repo/wire"
)

const repohost = "https://repo.gin.g-node.org"

// GetRepos Get a list of a user's repositories
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
