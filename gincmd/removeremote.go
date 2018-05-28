package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func rmRemote(cmd *cobra.Command, args []string) {
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}
	name := args[0]
	err := ginclient.RemoveRemote(name)
	CheckError(err)
	fmt.Printf(":: Remote removed: %s\n", name)
	defremote, _ := ginclient.DefaultRemote()
	if defremote == name {
		ginclient.SetDefaultRemote("")
		fmt.Printf(":: %s was the default remote. Current default is unset.\n:: Use 'gin use-remote' to set a new default.\n", name)
	}
}

// RemoveRemoteCmd sets up the 'remove-remote' repository subcommand
func RemoveRemoteCmd() *cobra.Command {
	description := `Remove a remote from the current repository.`

	args := map[string]string{
		"<name>": "The name of the remote",
	}
	var removeRemoteCmd = &cobra.Command{
		Use:                   "remove-remote <name>",
		Short:                 description,
		Long:                  formatdesc(description, args),
		Args:                  cobra.ExactArgs(1),
		Run:                   rmRemote,
		Aliases:               []string{"rm-remote"},
		DisableFlagsInUseLine: true,
	}
	return removeRemoteCmd
}
