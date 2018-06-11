package gincmd

import (
	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func getContent(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	conf := config.Read()
	// TODO: no need for client; use remotes (and all keys?)
	gincl := ginclient.New(conf.DefaultServer)
	requirelogin(cmd, gincl, !jsonout)
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}

	getcchan := make(chan git.RepoFileStatus)
	go gincl.GetContent(args, getcchan)
	formatOutput(getcchan, 0, jsonout)
}

// GetContentCmd sets up the 'get-content' subcommand
func GetContentCmd() *cobra.Command {
	description := "Download the content of the listed files. The get-content command is intended to be used to retrieve the content of placeholder files in a local repository. This command must be called from within the local repository clone. With no arguments, downloads the content for all files under the working directory, recursively."
	args := map[string]string{
		"<filenames>": "One or more directories or files to lock.",
	}
	var cmd = &cobra.Command{
		Use:                   "get-content [--json] [<filenames>]...",
		Short:                 "Download the content of files from a remote repository",
		Long:                  formatdesc(description, args),
		Args:                  cobra.ArbitraryArgs,
		Run:                   getContent,
		Aliases:               []string{"getc"},
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, "Print output in JSON format.")
	return cmd
}
