package gincmd

import (
	"fmt"

	"github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func lock(cmd *cobra.Command, args []string) {
	prStyle := determinePrintStyle(cmd)
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}
	if prStyle != psJSON {
		fmt.Println(":: Locking files")
	}
	// lock should do nothing in direct mode
	// NOTE: Direct mode repositories are deprecated, but we should still look
	// out for them
	if git.IsDirect() {
		fmt.Print("   Repository is in DIRECT mode: files are always unlocked")
		return
	}
	// TODO: need server config? Just use remotes
	conf := config.Read()
	gincl := ginclient.New(conf.DefaultServer)
	nitems := countItemsLockChange(args)
	lockchan := make(chan git.RepoFileStatus)

	go gincl.LockContent(args, lockchan)
	formatOutput(lockchan, prStyle, nitems)
}

// LockCmd sets up the file 'lock' subcommand
func LockCmd() *cobra.Command {
	description := "Lock one or more files after editing. After unlocking files for editing (using the 'unlock' command), it is recommended that they be locked again. This records any changes made and prepares a file for upload.\n\nLocked files are replaced by symbolic links in the working directory (where supported by the filesystem).\n\nAfter performing an 'upload', 'download', or 'get', affected files are reverted to a locked state.\n\nLocking a file takes longer depending on the size of the file."
	args := map[string]string{
		"<filenames>": "One or more directories or files to lock.",
	}
	var cmd = &cobra.Command{
		Use:                   "lock [--json | --verbose] <filenames>...",
		Short:                 "Lock files",
		Long:                  formatdesc(description, args),
		Args:                  cobra.MinimumNArgs(1),
		Run:                   lock,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, jsonHelpMsg)
	// cmd.Flags().Bool("verbose", false, verboseHelpMsg)
	return cmd
}
