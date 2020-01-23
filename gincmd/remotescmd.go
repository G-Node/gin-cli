package gincmd

import (
	"encoding/json"
	"fmt"

	"github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func printremotes(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	jsonout, _ := flags.GetBool("json")

	switch git.Checkwd() {
	case git.NotRepository:
		Die(ginerrors.NotInRepo)
	case git.NotAnnex:
		Warn(ginerrors.MissingAnnex)
	}

	gr := git.New(".")
	remotes, err := gr.RemoteShow()
	CheckError(err)
	defremote, err := ginclient.DefaultRemote()
	CheckError(err)
	if jsonout {
		remotesjson, _ := json.Marshal(remotes)
		fmt.Print(string(remotesjson))
	} else {
		fmt.Println(":: Configured remotes")
		for name, loc := range remotes {
			fmt.Printf(" %s: %s", name, loc)
			if name == defremote {
				fmt.Fprintf(color.Output, green(" [default]"))
			}
			fmt.Println()
		}
	}
}

// RemotesCmd sets up the 'remotes' subcommand
func RemotesCmd() *cobra.Command {
	description := `List configured remotes and their information.`
	var cmd = &cobra.Command{
		Use:                   "remotes",
		Short:                 "List the repository's configured remotes",
		Long:                  formatdesc(description, nil),
		Args:                  cobra.NoArgs,
		Run:                   printremotes,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, jsonHelpMsg)
	return cmd
}
