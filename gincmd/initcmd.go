package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/git"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func initRepo(cmd *cobra.Command, args []string) {
	gincl := ginclient.New(util.Config.GinHost)
	fmt.Print("Initialising local storage ")
	err := gincl.InitDir()
	util.CheckError(err)
	_, err = git.CommitIfNew("")
	util.CheckError(err)
	fmt.Println(green("OK"))
}

// InitCmd sets up the 'init' repository subcommand
func InitCmd() *cobra.Command {
	description := "Initialise a local repository in the current directory with the default options."
	var initRepoCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialise the current directory as a gin repository",
		Long:  formatdesc(description, nil),
		Args:  cobra.NoArgs,
		Run:   initRepo,
		DisableFlagsInUseLine: true,
	}
	return initRepoCmd
}
