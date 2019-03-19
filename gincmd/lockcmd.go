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
	description := "Lock one or more files to prevent editing. This changes the type of the file in the repository. A 'commit' command is required to save the change. Locked files that have not yet been committed are marked as 'Lock status changed' (short TC) in the output of the 'ls' command.\n\nLocked files are replaced by pointer files in the working directory (or symbolic links where supported by the filesystem).\n\nLocking a file takes longer depending on the size of the file."
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
