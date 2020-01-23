package gincmd

import (
	"fmt"

	"github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/howeyc/gopass"
	"github.com/spf13/cobra"
)

// login requests credentials, performs login with auth server, and stores the token.
func login(cmd *cobra.Command, args []string) {
	var username, password string

	flags := cmd.Flags()
	srvalias, _ := flags.GetString("server")

	conf := config.Read()
	if srvalias == "" {
		srvalias = conf.DefaultServer
	}
	fmt.Printf("Logging into %s\n", srvalias)

	if len(args) == 0 {
		// prompt for login
		fmt.Print("Login: ")
		fmt.Scanln(&username)
	} else {
		username = args[0]
	}

	// prompt for password
	fmt.Print("Password: ")
	pwbytes, err := gopass.GetPasswdMasked()
	fmt.Println()
	if err != nil {
		// read error or gopass.ErrInterrupted
		if err == gopass.ErrInterrupted {
			Die("Cancelled.")
		}
		if err == gopass.ErrMaxLengthExceeded {
			Die("Input too long")
		}
		Die(err)
	}

	password = string(pwbytes)
	if password == "" {
		Die("No password provided. Aborting.")
	}

	gincl := ginclient.New(srvalias)
	err = gincl.Login(username, password, "gin-cli")
	CheckError(err)
	info, err := gincl.RequestAccount(username)
	CheckError(err)
	name := info.FullName
	if name == "" {
		name = info.UserName
	}
	fmt.Printf(":: Welcome %s\n", name)
	fmt.Printf(":: Successfully logged into %s [%s]\n", srvalias, gincl.WebAddress())
}

// LoginCmd sets up the 'login' subcommand
func LoginCmd() *cobra.Command {
	description := "Login to the GIN services.\n\nIf no username is specified on the command line, you will be prompted for it. The login command always prompts for a password."
	var cmd = &cobra.Command{
		Use:                   "login [<username>]",
		Short:                 "Login to the GIN services",
		Long:                  formatdesc(description, nil),
		Args:                  cobra.MaximumNArgs(1),
		Run:                   login,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().String("server", "", "Specify server `alias` to log into. See also 'gin servers'.")
	return cmd
}
