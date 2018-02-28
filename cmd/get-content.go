package gincmd

import (
	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func getContent(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, !jsonout)
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	getcchan := make(chan ginclient.RepoFileStatus)
	go gincl.GetContent(args, getcchan)
	printProgress(getcchan, jsonout)
}

// GetContentCmd sets up the 'get-content' subcommand
func GetContentCmd() *cobra.Command {
	var getContentCmd = &cobra.Command{
		Use:                   "get-content [--json] [<filenames>]...",
		Short:                 "Download the content of files from a remote repository",
		Long:                  w.Wrap("Download the content of the listed files. The get-content command is intended to be used to retrieve the content of placeholder files in a local repository. This command must be called from within the local repository clone. With no arguments, downloads the content for all files under the working directory, recursively.", 80),
		Args:                  cobra.ArbitraryArgs,
		Run:                   getContent,
		Aliases:               []string{"getc"},
		DisableFlagsInUseLine: true,
	}
	getContentCmd.Flags().Bool("json", false, "Print output in JSON format.")
	return getContentCmd
}
