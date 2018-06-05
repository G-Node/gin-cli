package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/spf13/cobra"
)

func useServer(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		name := args[0]
		err := ginclient.SetDefaultServer(name)
		CheckError(err)
	}
	name, err := ginclient.DefaultServer()
	CheckError(err)
	fmt.Printf(":: Default server: %s\n", name)
}

// UseServerCmd sets up the 'use-server' subcommand
func UseServerCmd() *cobra.Command {
	description := `Set the default GIN server for user and repository management commands.

The following commands are affected by this setting:
create, info, keys, login, logout, repoinfo, repos

This setting can be overridden in each command by using the --server flag.

With no arguments, this command simply prints the currently configured default server.`
	args := map[string]string{
		"<name>": "The name of the server to use by default",
	}
	var addRemoteCmd = &cobra.Command{
		Use:   "use-server [<name>]",
		Short: "Set the default server for the client",
		Long:  formatdesc(description, args),
		Args:  cobra.MaximumNArgs(1),
		Run:   useServer,
		DisableFlagsInUseLine: true,
	}
	return addRemoteCmd
}
