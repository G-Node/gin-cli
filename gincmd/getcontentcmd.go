package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func getContent(cmd *cobra.Command, args []string) {
	prStyle := determinePrintStyle(cmd)
	conf := config.Read()
	// TODO: no need for client; use remotes (and all keys?)
	gincl := ginclient.New(conf.DefaultServer)
	requirelogin(cmd, gincl, prStyle != psJSON)
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}

	if prStyle == psDefault {
		fmt.Println(":: Downloading file content")
	}
	getcchan := make(chan git.RepoFileStatus)
	go gincl.GetContent(args, getcchan)
	formatOutput(getcchan, prStyle, 0)
}

// GetContentCmd sets up the 'get-content' subcommand
func GetContentCmd() *cobra.Command {
	description := "Download the content of the listed files. The get-content command is intended to be used to retrieve the content of placeholder files in a local repository. This command must be called from within the local repository clone. With no arguments, downloads the content for all files under the working directory, recursively."
	args := map[string]string{
		"<filenames>": "One or more directories or files to download.",
	}
	var cmd = &cobra.Command{
		Use:                   "get-content [--json | --verbose] [<filenames>]...",
		Short:                 "Download the content of files from a remote repository",
		Long:                  formatdesc(description, args),
		Args:                  cobra.ArbitraryArgs,
		Run:                   getContent,
		Aliases:               []string{"getc"},
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, jsonHelpMsg)
	cmd.Flags().Bool("verbose", false, verboseHelpMsg)
	return cmd
}
