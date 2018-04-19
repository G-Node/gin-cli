package gincmd

import (
	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func lock(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	gincl := ginclient.New(util.Config.GinHost)
	lockchan := make(chan ginclient.RepoFileStatus)
	go gincl.LockContent(args, lockchan)
	formatOutput(lockchan, jsonout)
}

// LockCmd sets up the file 'lock' subcommand
func LockCmd() *cobra.Command {
	description := "Lock one or more files after editing. After unlocking files for editing (using the 'unlock' command), it is recommended that they be locked again. This records any changes made and prepares a file for upload.\n\nLocked files are replaced by symbolic links in the working directory (where supported by the filesystem).\n\nAfter performing an 'upload', 'download', or 'get', affected files are reverted to a locked state.\n\nLocking a file takes longer depending on the size of the file."
	args := map[string]string{
		"<filenames>": "One or more directories or files to lock.",
	}
	var lockCmd = &cobra.Command{
		Use:   "lock [--json] [<filenames>]...",
		Short: "Lock files",
		Long:  formatdesc(description, args),
		Args:  cobra.ArbitraryArgs,
		Run:   lock,
		DisableFlagsInUseLine: true,
	}
	lockCmd.Flags().Bool("json", false, "Print output in JSON format.")
	return lockCmd
}
