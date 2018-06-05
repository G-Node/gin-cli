package gincmd

import (
	"fmt"

	"github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/spf13/cobra"
)

func rmServer(cmd *cobra.Command, args []string) {
	alias := args[0]
	defserver := config.Read().DefaultServer
	err := ginclient.RemoveServer(alias)
	CheckError(err)
	fmt.Printf(":: Server removed: %s\n", alias)
	if alias == "gin" {
		fmt.Println(":: 'gin' alias now refers to the built-in G-Node GIN server.")
	} else if defserver == alias {
		config.SetDefaultServer("gin")
		fmt.Printf(":: %s was the default sever. Reverting to default server 'gin'.\n:: Use 'gin use-server' to set a new default.\n", alias)
	}
}

// RemoveServerCmd sets up the 'remove-server' repository subcommand
func RemoveServerCmd() *cobra.Command {
	description := `Remove a server from the global configuration.`

	args := map[string]string{
		"<alias>": "The alias (name) of the server in the configuration",
	}
	var cmd = &cobra.Command{
		Use:                   "remove-server <alias>",
		Short:                 "Remove a server from the global configuration",
		Long:                  formatdesc(description, args),
		Args:                  cobra.ExactArgs(1),
		Run:                   rmServer,
		Aliases:               []string{"rm-server"},
		DisableFlagsInUseLine: true,
	}
	return cmd
}
