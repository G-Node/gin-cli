package gincmd

import (
	"bytes"
	"encoding/json"
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/spf13/cobra"
)

func printAccountInfo(cmd *cobra.Command, args []string) {
	var username string

	flags := cmd.Flags()
	jsonout, _ := flags.GetBool("json")
	srvalias, _ := flags.GetString("server")

	conf := config.Read()
	if srvalias == "" {
		srvalias = conf.DefaultServer
	}
	gincl := ginclient.New(srvalias)
	gincl.LoadToken() // does not REQUIRE login

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
	CheckError(err)

	var outBuffer bytes.Buffer
	if jsonout {
		infojson, _ := json.Marshal(info)
		outBuffer.Write(infojson)
	} else {
		_, _ = outBuffer.WriteString(fmt.Sprintf("User %s\nName: %s\n", info.UserName, info.FullName))
		if info.Email != "" {
			_, _ = outBuffer.WriteString(fmt.Sprintf("Email: %s\n", info.Email))
		}
	}

	fmt.Println(outBuffer.String())
}

// InfoCmd sets up the  user 'info' subcommand
func InfoCmd() *cobra.Command {
	description := "Print user information. If no argument is provided, it will print the information of the currently logged in user. Using this command with no argument can also be used to check if a user is currently logged in."
	args := map[string]string{
		"<username>": "The name of the user whose information should be printed. This can be the username of the currently logged in user (default), in which case the command will print all the profile information with indicators for which data is publicly visible. If it is the username of a different user, only the publicly visible information is printed.",
	}
	var cmd = &cobra.Command{
		Use:                   "info [username]",
		Short:                 "Print a user's information",
		Long:                  formatdesc(description, args),
		Args:                  cobra.MaximumNArgs(1),
		Run:                   printAccountInfo,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().String("server", "", "Specify server `alias` for info lookup. See also 'gin servers'.")
	cmd.Flags().Bool("json", false, jsonHelpMsg)
	return cmd
}
