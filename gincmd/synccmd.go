package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func sync(cmd *cobra.Command, args []string) {
	prStyle := determinePrintStyle(cmd)
	// TODO: no client necessary? Just use remotes
	conf := config.Read()
	gincl := ginclient.New(conf.DefaultServer)
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}

	content, _ := cmd.Flags().GetBool("content")
	if prStyle == psDefault {
		fmt.Print(":: Synchronising changes ")
	}
	err := gincl.Sync(content)
	CheckError(err)
	if prStyle == psDefault {
		fmt.Fprintln(color.Output, green("OK"))
	}
}

// SyncCmd sets up the 'sync' subcommand
func SyncCmd() *cobra.Command {
	description := "Synchronises changes bidirectionally between remote repositories and the local clone. This will create new files that were added remotely, delete files that were removed, and update files that were changed.\n\nOptionally downloads and uploads the content of all files in the repository. If 'content' is not specified, new files will be empty placeholders. Content of individual files can later be retrieved using the 'get-content' command."
	var cmd = &cobra.Command{
		Use:                   "sync [--json | --verbose] [--content]",
		Short:                 "Sync all new information bidirectionally between local and remote repositories",
		Long:                  formatdesc(description, nil),
		Args:                  cobra.NoArgs,
		Run:                   sync,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, jsonHelpMsg)
	// cmd.Flags().Bool("verbose", false, verboseHelpMsg)
	cmd.Flags().Bool("content", false, "Download and upload the content for all files in the repository.")
	return cmd
}
