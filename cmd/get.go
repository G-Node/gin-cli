package gincmd

import (
	"fmt"
	"strings"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func isValidRepoPath(path string) bool {
	return strings.Contains(path, "/")
}

func getRepo(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	repostr := args[0]
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, !jsonout)

	if !isValidRepoPath(repostr) {
		util.Die(fmt.Sprintf("Invalid repository path '%s'. Full repository name should be the owner's username followed by the repository name, separated by a '/'.\nType 'gin help get' for information and examples.", repostr))
	}

	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	clonechan := make(chan ginclient.RepoFileStatus)
	go gincl.CloneRepo(repostr, clonechan)
	printProgress(clonechan, jsonout)
}

func GetCmd() *cobra.Command {
	var getRepoCmd = &cobra.Command{
		Use:   "get [--json] <repository>",
		Short: "Retrieve (clone) a repository from the remote server",
		Long:  "Download a remote repository to a new directory and initialise the directory with the default options. The local directory is referred to as the 'clone' of the repository.",
		Args:  cobra.ExactArgs(1),
		Run:   getRepo,
		DisableFlagsInUseLine: true,
	}
	getRepoCmd.Flags().Bool("json", false, "Print output in JSON format.")
	return getRepoCmd
}
