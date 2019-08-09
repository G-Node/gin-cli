package gincmd

import (
	"fmt"
	"strings"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/spf13/cobra"
)

func isValidRepoPath(path string) bool {
	return strings.Contains(path, "/")
}

func getRepo(cmd *cobra.Command, args []string) {
	prStyle := determinePrintStyle(cmd)
	srvalias, _ := cmd.Flags().GetString("server")
	conf := config.Read()
	if srvalias == "" {
		srvalias = conf.DefaultServer
	}
	repostr := args[0]
	gincl := ginclient.New(srvalias)
	requirelogin(cmd, gincl, prStyle != psJSON)

	if !isValidRepoPath(repostr) {
		Die(fmt.Sprintf("Invalid repository path '%s'. Full repository name should be the owner's username followed by the repository name, separated by a '/'.\nType 'gin help get' for information and examples.", repostr))
	}

	clonechan := gincl.CloneRepo(repostr)
	formatOutput(clonechan, prStyle, 0)
	defaultRemoteIfUnset("origin")
	new, err := ginclient.CommitIfNew()
	if new {
		// Push the new commit to initialise origin
		uploadchan := gincl.Upload(nil, []string{"origin"})
		for range uploadchan {
			// Wait for channel to close
		}
	}
	CheckError(err)
}

// GetCmd sets up the 'get' repository subcommand
func GetCmd() *cobra.Command {
	description := "Download a remote repository to a new directory and initialise the directory with the default options. The local directory is referred to as the 'clone' of the repository."
	args := map[string]string{
		"<repopath>": "The repository path must be specified on the command line. A repository path is the owner's username, followed by a \"/\" and the repository name.",
	}
	examples := map[string]string{
		"Get and initialise the repository named 'example' owned by user 'alice'": "$ gin get alice/example",
		"Get and initialise the repository named 'eegdata' owned by user 'peter'": "$ gin get peter/eegdata",
	}
	var cmd = &cobra.Command{
		// Use:                   "get [--json | --verbose] <repopath>",
		Use:                   "get [--json] <repopath>",
		Short:                 "Retrieve (clone) a repository from the remote server",
		Long:                  formatdesc(description, args),
		Example:               formatexamples(examples),
		Args:                  cobra.ExactArgs(1),
		Run:                   getRepo,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, jsonHelpMsg)
	// cmd.Flags().Bool("verbose", false, verboseHelpMsg)
	cmd.Flags().String("server", "", "Specify server `alias` for the repository. See also 'gin servers'.")
	return cmd
}
