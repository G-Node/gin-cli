package repo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/G-Node/gin-cli/auth"
	"github.com/G-Node/gin-cli/client"
	"github.com/G-Node/gin-repo/wire"
)

const repohost = "https://repo.gin.g-node.org"

// GetRepos Get a list of all public repositories
func GetRepos() ([]wire.Repo, error) {
	_, token, err := auth.LoadToken(false)

	repocl := client.NewClient(repohost)
	repocl.Token = token
	res, err := repocl.Get("/repos/public")

	if err != nil {
		return nil, fmt.Errorf("Request for repos returned error: %s", err)
	} else if res.StatusCode != 200 {
		return nil, fmt.Errorf("[Repo listing error] Server returned: %s", res.Status)
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
