package gincmd

import (
	"fmt"
	"os"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func annexrun(cmd *cobra.Command, args []string) {
	gr := git.New(".")
	gr.SSHCmd = ginclient.SSHOpts()
	annexcmd := gr.AnnexCommand(args...)
	err := annexcmd.Start()
	CheckError(err)
	var line string
	var rerr error
	for rerr = nil; rerr == nil; line, rerr = annexcmd.OutReader.ReadString('\n') {
		fmt.Print(line)
	}
	for rerr = nil; rerr == nil; line, rerr = annexcmd.ErrReader.ReadString('\n') {
		fmt.Fprint(os.Stderr, line)
	}
	if annexcmd.Wait() != nil {
		os.Exit(1)
	}
}

// AnnexCmd sets up the 'annex' passthrough subcommand
func AnnexCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:                   "annex <cmd> [<args>]...",
		Short:                 "Run a 'git annex' command through the gin client",
		Long:                  "",
		Args:                  cobra.ArbitraryArgs,
		Run:                   annexrun,
		DisableFlagsInUseLine: true,
		Hidden:                true,
		DisableFlagParsing:    true,
	}
	return cmd
}
