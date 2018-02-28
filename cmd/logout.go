package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func logout(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		usageDie(cmd)
	}
	gincl := ginclient.NewClient(util.Config.GinHost)
	err := gincl.LoadToken()
	if err != nil {
		util.Die("You are not logged in.")
	}

	gincl.Logout()
	fmt.Println("You have been logged out.")
}

func LogoutCmd() *cobra.Command {
	var logoutCmd = &cobra.Command{
		Use:   "logout",
		Short: "Logout of the GIN services",
		Long:  "Logout of the GIN services. This command takes no arguments.",
		Args:  cobra.NoArgs,
		Run:   logout,
		DisableFlagsInUseLine: true,
	}
	return logoutCmd
}
