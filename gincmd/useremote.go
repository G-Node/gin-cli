package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func setRemote(cmd *cobra.Command, args []string) {
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}
	name := args[0]
	err := ginclient.SetDefaultRemote(name)
	CheckError(err)
	fmt.Printf(":: Default remote: %s\n", name)
}

// UseRemoteCmd sets up the 'use-remote' repository subcommand
func UseRemoteCmd() *cobra.Command {
	description := "Set the default remote for uploading. The default remote is used when 'gin upload' is invoked without specifying the --to option."
	args := map[string]string{
		"<name>": "The name of the remote to use by default",
	}
	var addRemoteCmd = &cobra.Command{
		Use:   "use-remote <name>",
		Short: "Set the repository's default upload remote",
		Long:  formatdesc(description, args),
		Args:  cobra.ExactArgs(1),
		Run:   setRemote,
		DisableFlagsInUseLine: true,
	}
	return addRemoteCmd
}
