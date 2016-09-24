package repo

import (
	"fmt"

	"github.com/G-Node/gin-cli/auth"
	"github.com/G-Node/gin-cli/client"
)

const repohost = "https://repo.gin.g-node.org"

// GetRepos Get a list of all public repositories
func GetRepos() error {
	_, token, err := auth.LoadToken(false)

	repocl := client.NewClient(repohost)
	repocl.Token = token
	res, err := repocl.Get("/repos/public")

	if err != nil {
		return fmt.Errorf("Request for repos returned error: %s", err)
	} else if res.StatusCode != 200 {
		return fmt.Errorf("[Repo listing error] Server returned: %s", res.Status)
	}

	defer client.CloseRes(res.Body)
	// b, err := ioutil.ReadAll(res.Body)
	// TODO: make slice of repo structs and return
	return nil
}
