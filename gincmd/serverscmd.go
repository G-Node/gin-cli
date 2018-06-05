package gincmd

import (
	"fmt"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func servers(cmd *cobra.Command, args []string) {
	conf := config.Read()
	servermap := conf.Servers
	defserver := conf.DefaultServer

	fmt.Println(":: Configured servers")
	for alias, srvcfg := range servermap {
		fmt.Printf("* %s", alias)
		if alias == defserver {
			fmt.Fprintf(color.Output, green(" [default]"))
		}
		fmt.Println()
		fmt.Printf("  web: %s\n", srvcfg.Web.AddressStr())
		fmt.Printf("  git: %s\n\n", srvcfg.Git.AddressStr())
	}
}

// ServersCmd sets up the 'servers' subcommand
func ServersCmd() *cobra.Command {
	description := `List globally configured servers and their information.`
	var serversCmd = &cobra.Command{
		Use:   "servers",
		Short: "List the globally configured servers",
		Long:  formatdesc(description, nil),
		Args:  cobra.NoArgs,
		Run:   servers,
		DisableFlagsInUseLine: true,
	}
	return serversCmd
}
