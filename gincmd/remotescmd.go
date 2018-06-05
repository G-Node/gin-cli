package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func remotes(cmd *cobra.Command, args []string) {
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}

	remotes, err := git.RemoteShow()
	CheckError(err)
	defremote, err := ginclient.DefaultRemote()
	CheckError(err)
	fmt.Println(":: Configured remotes")
	for name, loc := range remotes {
		fmt.Printf(" %s: %s", name, loc)
		if name == defremote {
			fmt.Fprintf(color.Output, green(" [default]"))
		}
		fmt.Println()
	}
}

// RemotesCmd sets up the 'remotes' subcommand
func RemotesCmd() *cobra.Command {
	description := `List configured remotes and their information.`
	var cmd = &cobra.Command{
		Use:   "remotes",
		Short: "List the repository's configured remotes",
		Long:  formatdesc(description, nil),
		Args:  cobra.NoArgs,
		Run:   remotes,
		DisableFlagsInUseLine: true,
	}
	return cmd
}
