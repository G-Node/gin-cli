package gincmd

import (
	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func unlock(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	gincl := ginclient.NewClient(util.Config.GinHost)
	unlockchan := make(chan ginclient.RepoFileStatus)
	go gincl.UnlockContent(args, unlockchan)
	formatOutput(unlockchan, jsonout)
}

// UnlockCmd sets up the file 'unlock' subcommand
func UnlockCmd() *cobra.Command {
	description := "Unlock one or more files for editing. Files added to the repository are left in a locked state, which allows reading but prevents editing. In order to edit or write to a file, it must first be unlocked. When done editing, it is recommended that the file be locked again using the 'lock' command.\n\nAfter performing an 'upload, 'download', or 'get', affected files are always reverted to the locked state.\n\nUnlocking a file takes longer depending on its size."
	args := map[string]string{
		"<filenames>": "One or more directories or files to unllock.",
	}
	var unlockCmd = &cobra.Command{
		Use:   "unlock [--json] [<filenames>]...",
		Short: "Unlock files for editing",
		Long:  formatdesc(description, args),
		Args:  cobra.ArbitraryArgs,
		Run:   unlock,
		DisableFlagsInUseLine: true,
	}
	unlockCmd.Flags().Bool("json", false, "Print output in JSON format.")
	return unlockCmd
}
