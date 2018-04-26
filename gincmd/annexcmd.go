package gincmd

import (
	"fmt"
	"os"

	"github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func annexrun(cmd *cobra.Command, args []string) {
	conf := config.Read()
	gincl := ginclient.New(conf.GinHost)
	_ = gincl.LoadToken() // OK to run without token
	annexcmd := git.AnnexCommand(args...)
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
	var annexCmd = &cobra.Command{
		Use:   "annex <cmd> [<args>]...",
		Short: "Run a 'git annex' command through the gin client",
		Long:  "",
		Args:  cobra.ArbitraryArgs,
		Run:   annexrun,
		DisableFlagsInUseLine: true,
		Hidden:                true,
		DisableFlagParsing:    true,
	}
	return annexCmd
}
