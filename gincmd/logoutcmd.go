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
	flags := cmd.Flags()
	srvalias, _ := flags.GetString("server")

	conf := config.Read()
	if srvalias == "" {
		srvalias = conf.DefaultServer
	}
	gincl := ginclient.New(srvalias)
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
	cmd.Flags().String("server", "", "Specify server `alias` where the repository will be created. See also 'gin servers'.")
	return cmd
}
