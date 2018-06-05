package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/spf13/cobra"
)

func logout(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		usageDie(cmd)
	}
	conf := config.Read()
	srvcfg := conf.Servers["gin"] // TODO: Support aliases
	gincl := ginclient.New(srvcfg.Web.AddressStr())
	err := gincl.LoadToken()
	if err != nil {
		Die("You are not logged in.")
	}

	gincl.Logout()
	fmt.Println(":: You have been logged out.")
}

// LogoutCmd sets up the 'logout' subcommand
func LogoutCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "logout",
		Short: "Logout of the GIN services",
		Long:  "Logout of the GIN services.\n\nThis command takes no arguments.",
		Args:  cobra.NoArgs,
		Run:   logout,
		DisableFlagsInUseLine: true,
	}
	return cmd
}
