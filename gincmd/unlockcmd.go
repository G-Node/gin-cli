package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func countItemsLockChange(paths []string) (count int) {
	// BUG: Miscalculates number in some cases
	wichan := make(chan git.AnnexWhereisRes)
	go git.AnnexWhereis(paths, wichan)
	for range wichan {
		count++
	}
	return
}

func unlock(cmd *cobra.Command, args []string) {
	prStyle := determinePrintStyle(cmd)
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}

	if prStyle != psJSON {
		fmt.Println(":: Unlocking files")
	}
	// unlock should do nothing in direct mode
	if git.IsDirect() {
		fmt.Print("   Repository is in DIRECT mode: files are always unlocked")
		return
	}

	// TODO: Probably doesn't need a server config
	conf := config.Read()
	defserver := conf.DefaultServer
	gincl := ginclient.New(defserver)
	nitems := countItemsLockChange(args)
	unlockchan := make(chan git.RepoFileStatus)
	go gincl.UnlockContent(args, unlockchan)
	formatOutput(unlockchan, prStyle, nitems)
}

// UnlockCmd sets up the file 'unlock' subcommand
func UnlockCmd() *cobra.Command {
	description := "Unlock one or more files to allow editing. This changes the type of the file in the repository. A 'commit' command is required to save the change. Unmodified unlocked files that have not yet been committed are marked as 'Lock status changed' (short TC) in the output of the 'ls' command.\n\nUnlocking a file takes longer depending on its size."
	args := map[string]string{
		"<filenames>": "One or more directories or files to unlock.",
	}
	var cmd = &cobra.Command{
		// Use:                   "unlock [--json | --verbose] <filenames>...",
		Use:                   "unlock [--json] <filenames>...",
		Short:                 "Unlock files for editing",
		Long:                  formatdesc(description, args),
		Args:                  cobra.MinimumNArgs(1),
		Run:                   unlock,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, jsonHelpMsg)
	// cmd.Flags().Bool("verbose", false, verboseHelpMsg)
	return cmd
}
