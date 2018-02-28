package gincmd

import (
	"bytes"
	"fmt"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func printAccountInfo(cmd *cobra.Command, args []string) {
	var username string

	gincl := ginclient.NewClient(util.Config.GinHost)
	_ = gincl.LoadToken() // does not REQUIRE login

	if len(args) == 0 {
		username = gincl.Username
	} else {
		username = args[0]
	}

	if username == "" {
		// prompt for username
		fmt.Print("Specify username for info lookup: ")
		username = ""
		fmt.Scanln(&username)
	}

	info, err := gincl.RequestAccount(username)
	util.CheckError(err)

	var outBuffer bytes.Buffer
	_, _ = outBuffer.WriteString(fmt.Sprintf("User %s\nName: %s\n", info.UserName, info.FullName))
	if info.Email != "" {
		_, _ = outBuffer.WriteString(fmt.Sprintf("Email: %s\n", info.Email))
	}

	fmt.Println(outBuffer.String())
}

func InfoCmd() *cobra.Command {
	var infoCmd = &cobra.Command{
		Use:   "info [username]",
		Short: "Print a user's information",
		Long:  "Print user information. If no argument is provided, it will print the information of the currently logged in user. Using this command with no argument can also be used to check if a user is currently logged in.",
		Args:  cobra.MaximumNArgs(1),
		Run:   printAccountInfo,
		DisableFlagsInUseLine: true,
	}
	return infoCmd
}
