package gincmd

import (
	"github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func countItemsLock(paths []string) (count int) {
	statchan := make(chan git.AnnexStatusRes)
	go git.AnnexStatus(paths, statchan)
	for _ = range statchan {
		count++
	}
	return
}

func lock(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}
	// lock should do nothing in direct mode
	if git.IsDirect() {
		return
	}
	conf := config.Read()
	gincl := ginclient.New(conf.GinHost)
	nitems := countItemsLock(args)
	lockchan := make(chan git.RepoFileStatus)
	go gincl.LockContent(args, lockchan)
	formatOutput(lockchan, nitems, jsonout)
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
