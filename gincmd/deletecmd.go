package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/spf13/cobra"
)

func deleteRepo(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	srvalias, _ := flags.GetString("server")

	conf := config.Read()
	if srvalias == "" {
		srvalias = conf.DefaultServer
	}
	gincl := ginclient.New(srvalias)
	requirelogin(cmd, gincl, true)
	var repostr, confirmation string

	if len(args) == 0 {
		usageDie(cmd)
	} else {
		repostr = args[0]
	}

	repoinfo, err := gincl.GetRepo(repostr)
	CheckError(err)

	if repoinfo.FullName != repostr {
		log.Write("ERROR: Mismatch in repository names: %s != %s", repoinfo.FullName, repostr)
		Die("An unexpected error occurred while communicating with the server.")
	}

	fmt.Println("--- WARNING ---")
	fmt.Println("You are about to delete a remote repository, all its files, and history.")
	fmt.Println("This action is irreversible.")
	fmt.Printf("You are about to delete the repository at: %s\n", repoinfo.HTMLURL)

	fmt.Println("If you are sure you want to delete this repository, type its full name (owner/name) below")
	fmt.Print("> ")
	fmt.Scanln(&confirmation)

	if repoinfo.FullName == confirmation && repostr == confirmation {
		err = gincl.DelRepo(repostr)
		CheckError(err)
	} else {
		Die("Confirmation does not match repository name. Cancelling.")
	}

	fmt.Printf("Repository %s has been deleted!\n", repostr)
}

// DeleteCmd sets up the 'delete' repository subcommand
func DeleteCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "delete <repository>",
		Short: "Delete a repository from the GIN server",
		Long:  "Delete a repository from the GIN server.",
		Args:  cobra.ExactArgs(1),
		Run:   deleteRepo,
		DisableFlagsInUseLine: true,
		Hidden:                true,
	}
	cmd.Flags().String("server", "", "Specify server `alias` on which the repository to be deleted resides. See also 'gin servers'.")
	return cmd
}
