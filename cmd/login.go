package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/howeyc/gopass"
	"github.com/spf13/cobra"
)

// login requests credentials, performs login with auth server, and stores the token.
func login(cmd *cobra.Command, args []string) {
	var username string
	var password string

	if args != nil && len(args) == 0 {
		// prompt for login
		fmt.Print("Login: ")
		fmt.Scanln(&username)
	} else if len(args) > 1 {
		usageDie(cmd)
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
			util.Die("Cancelled.")
		}
		if err == gopass.ErrMaxLengthExceeded {
			util.Die("Input too long")
		}
		util.Die(err)
	}

	password = string(pwbytes)

	if password == "" {
		util.Die("No password provided. Aborting.")
	}

	gincl := ginclient.NewClient(util.Config.GinHost)
	err = gincl.Login(username, password, "gin-cli")
	util.CheckError(err)
	info, err := gincl.RequestAccount(username)
	util.CheckError(err)
	fmt.Printf("Hello %s. You are now logged in.\n", info.UserName)
}

func LoginCmd() *cobra.Command {
	var loginCmd = &cobra.Command{
		Use:   "login [<username>]",
		Short: "Login to the GIN services",
		Long:  "Login to the GIN services.",
		Args:  cobra.MaximumNArgs(1),
		Run:   login,
		DisableFlagsInUseLine: true,
	}
	return loginCmd
}
