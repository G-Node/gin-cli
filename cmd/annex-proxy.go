package gincmd

import (
	"fmt"
	"os"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func annexrun(cmd *cobra.Command, args []string) {
	gincl := ginclient.NewClient(util.Config.GinHost)
	_ = gincl.LoadToken() // OK to run without token
	annexcmd, err := ginclient.RunAnnexCommand(args...)
	util.CheckError(err)
	var line string
	for {
		line, err = annexcmd.OutPipe.ReadLine()
		if err != nil {
			break
		}
		fmt.Print(line)
	}
	fmt.Print(annexcmd.ErrPipe.ReadAll())
	err = annexcmd.Wait()
	if err != nil {
		os.Exit(1)
	}
}

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
