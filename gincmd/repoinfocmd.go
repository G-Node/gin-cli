package gincmd

import (
	"encoding/json"
	"fmt"
	"strings"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	gogs "github.com/gogits/go-gogs-client"
	"github.com/spf13/cobra"
)

func printRepoInfo(repo gogs.Repository) {
	fmt.Printf("* %s\n", repo.FullName)
	fmt.Printf("\tLocation: %s\n", repo.HTMLURL)
	desc := strings.Trim(repo.Description, "\n")
	if desc != "" {
		fmt.Printf("\tDescription: %s\n", desc)
	}
	if repo.Website != "" {
		fmt.Printf("\tWebsite: %s\n", repo.Website)
	}
	if !repo.Private {
		fmt.Println("\tThis repository is public")
	}
	fmt.Println()
}

func repoinfo(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	jsonout, _ := flags.GetBool("json")
	gincl := ginclient.New(util.Config.GinHost)
	requirelogin(cmd, gincl, true)
	repoinfo, err := gincl.GetRepo(args[0])
	util.CheckError(err)

	if jsonout {
		j, _ := json.Marshal(repoinfo)
		fmt.Println(string(j))
		return
	}
	printRepoInfo(repoinfo)
}

// RepoInfoCmd sets up the 'repoinfo' listing subcommand
func RepoInfoCmd() *cobra.Command {
	description := "Show the information for a specific repository on the server.\n\nThis can be used to check if the logged in user has access to a specific repository."

	args := map[string]string{
		"<repopath>": "The repository path must be specified on the command line. A repository path is the owner's username, followed by a \"/\" and the repository name.",
	}
	var reposCmd = &cobra.Command{
		Use:   "repoinfo --json <repopath>",
		Short: "Show the information for a specific repository",
		Long:  formatdesc(description, args),
		Args:  cobra.ExactArgs(1),
		Run:   repoinfo,
		DisableFlagsInUseLine: true,
	}
	reposCmd.Flags().Bool("json", false, "Print information in JSON format.")
	return reposCmd
}
