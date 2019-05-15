package gincmd

import (
	"encoding/json"
	"fmt"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type srvcfgWithDefault struct {
	config.ServerCfg
	Default bool
}

func servers(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	jsonout, _ := flags.GetBool("json")
	conf := config.Read()
	servermap := conf.Servers
	defserver := conf.DefaultServer

	// augment servermap with Default field
	serverdefaultmap := make(map[string]srvcfgWithDefault, len(servermap))
	for alias, srvcfg := range servermap {
		serverdefaultmap[alias] = srvcfgWithDefault{srvcfg, defserver == alias}
	}

	if jsonout {
		serversjson, _ := json.Marshal(serverdefaultmap)
		fmt.Print(string(serversjson))
	} else {
		fmt.Println(":: Configured servers")
		for alias, srvcfg := range serverdefaultmap {
			fmt.Printf("* %s", alias)
			if srvcfg.Default {
				fmt.Fprintf(color.Output, green(" [default]"))
			}
			fmt.Println()
			fmt.Printf("  web: %s\n", srvcfg.Web.AddressStr())
			fmt.Printf("  git: %s\n\n", srvcfg.Git.AddressStr())
		}
	}
}

// ServersCmd sets up the 'servers' subcommand
func ServersCmd() *cobra.Command {
	description := `List globally configured servers and their information.`
	var cmd = &cobra.Command{
		Use:                   "servers",
		Short:                 "List the globally configured servers",
		Long:                  formatdesc(description, nil),
		Args:                  cobra.NoArgs,
		Run:                   servers,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, jsonHelpMsg)
	return cmd
}
