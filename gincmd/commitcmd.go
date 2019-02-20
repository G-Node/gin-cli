package gincmd

import (
	"fmt"
	"os"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func commit(cmd *cobra.Command, args []string) {
	verbose, _ := cmd.Flags().GetBool("verbose")
	jsonout, _ := cmd.Flags().GetBool("json")
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}

	paths := args
	addchan := make(chan git.RepoFileStatus)
	go ginclient.Add(paths, addchan)
	formatOutput(addchan, 0, jsonout, verbose)

	if !jsonout && !verbose {
		fmt.Print(":: Recording changes ")
	}
	if verbose {
		fmt.Printf("Running Gin Command: %v \n", cmd.Name())
	}
	err := git.Commit(makeCommitMessage("commit", paths))
	var stat string
	if err != nil {
		if err.Error() == "Nothing to commit" {
			stat = "N/A\n:: No changes recorded"
		} else {
			Die(err)
		}
	} else {
		stat = green("OK")
	}
	if !jsonout && !verbose {
		fmt.Fprintln(color.Output, stat)
	}
}

func makeCommitMessage(action string, paths []string) (commitmsg string) {
	// add header commit line
	hostname, err := os.Hostname()
	if err != nil {
		log.Write("Could not retrieve hostname")
		hostname = unknownhostname
	}
	changes, err := git.DescribeIndexShort(paths)
	if err != nil {
		log.Write("Failed to determine changes for commit message")
		changes = ""
	}
	commitmsg = fmt.Sprintf("gin %s from %s\n\n%s", action, hostname, changes)
	return
}

// CommitCmd sets up the 'commit' subcommand
func CommitCmd() *cobra.Command {
	description := "Record changes made in a local repository. This command must be called from within the local repository clone. Specific files or directories may be specified. All changes made to the files and directories that are specified will be recorded, including addition of new files, modifications and renaming of existing files, and file deletions.\n\nIf no arguments are specified, no changes are recorded."
	args := map[string]string{"<filenames>": "One or more directories or files to commit."}
	var cmd = &cobra.Command{
		Use:                   "commit [--json | --verbose] [<filenames>]...",
		Short:                 "Record changes in local repository",
		Long:                  formatdesc(description, args),
		Args:                  cobra.ArbitraryArgs,
		Run:                   commit,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, "Print output in JSON format.")
	cmd.Flags().Bool("verbose", false, "Print raw information from git and git-annex commands directly.")
	return cmd
}
