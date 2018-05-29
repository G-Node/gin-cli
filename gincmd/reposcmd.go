package gincmd

import (
	"encoding/json"
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	gogs "github.com/gogits/go-gogs-client"
	"github.com/spf13/cobra"
)

func printRepoList(repolist []gogs.Repository) {
	for _, repo := range repolist {
		printRepoInfo(repo)
	}
}

func repos(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	jsonout, _ := flags.GetBool("json")
	allrepos, _ := flags.GetBool("all")
	sharedrepos, _ := flags.GetBool("shared")
	if (allrepos && sharedrepos) || ((allrepos || sharedrepos) && len(args) > 0) {
		usageDie(cmd)
	}
	conf := config.Read()
	srvcfg := conf.Servers["gin"] // TODO: Support aliases
	gincl := ginclient.New(srvcfg.Web.AddressStr())
	requirelogin(cmd, gincl, true)
	username := gincl.Username
	if len(args) == 1 && args[0] != username {
		username = args[0]
		// for other users, print everything
		allrepos = true
	}
	repolist, err := gincl.ListRepos(username)
	CheckError(err)

	var userrepos []gogs.Repository
	var otherrepos []gogs.Repository

	for _, repo := range repolist {
		if repo.Owner.UserName == gincl.Username {
			userrepos = append(userrepos, repo)
		} else {
			otherrepos = append(otherrepos, repo)
		}
	}

	if jsonout {
		var outlist []gogs.Repository
		if allrepos {
			outlist = append(userrepos, otherrepos...)
		} else if sharedrepos {
			outlist = otherrepos
		} else {
			outlist = userrepos
		}
		if len(outlist) > 0 {
			j, _ := json.Marshal(outlist)
			fmt.Println(string(j))
		}
		return
	}

	printedlines := 0
	if len(userrepos) > 0 && !sharedrepos {
		printedlines += len(userrepos)
		printRepoList(userrepos)
	}
	if len(otherrepos) > 0 && (sharedrepos || allrepos) {
		printedlines += len(otherrepos)
		printRepoList(otherrepos)
	}

	if printedlines == 0 {
		fmt.Println("No repositories found")
	}
}

// ReposCmd sets up the 'repos' listing subcommand
func ReposCmd() *cobra.Command {
	description := "List repositories on the server that provide read access. If no argument is provided, it will list the repositories owned by the logged in user.\n\nNote that only one of the options can be specified."

	args := map[string]string{
		"<username>": "The name of the user whose repositories should be listed. The list consists of public repositories and repositories shared with the logged in user.",
	}
	var reposCmd = &cobra.Command{
		Use:   "repos [--shared | --all | <username>]",
		Short: "List available remote repositories",
		Long:  formatdesc(description, args),
		Args:  cobra.MaximumNArgs(1),
		Run:   repos,
		DisableFlagsInUseLine: true,
	}
	reposCmd.Flags().Bool("all", false, "List all repositories accessible to the logged in user.")
	reposCmd.Flags().Bool("shared", false, "List all repositories that the user is a member of (excluding own repositories).")
	reposCmd.Flags().Bool("json", false, "Print listing in JSON format.")
	return reposCmd
}
