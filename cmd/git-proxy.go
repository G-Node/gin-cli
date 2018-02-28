package gincmd

import (
	"fmt"
	"os"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func gitrun(cmd *cobra.Command, args []string) {
	gincl := ginclient.NewClient(util.Config.GinHost)
	_ = gincl.LoadToken() // OK to run without token

	gitcmd, err := ginclient.RunGitCommand(args...)
	util.CheckError(err)
	for {
		line, readerr := gitcmd.OutPipe.ReadLine()
		if readerr != nil {
			break
		}
		fmt.Print(line)
	}
	fmt.Print(gitcmd.ErrPipe.ReadAll())
	err = gitcmd.Wait()
	if err != nil {
		os.Exit(1)
	}
}

func GitCmd() *cobra.Command {
	var gitCmd = &cobra.Command{
		Use:   "git <cmd> [<args>]...",
		Short: "Run a 'git' command through the gin client",
		Long:  "",
		Args:  cobra.ArbitraryArgs,
		Run:   gitrun,
		DisableFlagsInUseLine: true,
		Hidden:                true,
		DisableFlagParsing:    true,
	}
	return gitCmd
}
