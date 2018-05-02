package gincmd

import (
	"fmt"
	"os"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func commit(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	if !git.IsRepo() {
		Die("This command must be run from inside a gin repository.")
	}

	paths := args
	addchan := make(chan git.RepoFileStatus)
	go ginclient.Add(paths, addchan)
	formatOutput(addchan, 0, jsonout)

	if !jsonout {
		fmt.Print(":: Recording changes ")
	}
	err := git.Commit(makeCommitMessage("commit", paths))
	if err != nil {
		Die(err)
	}
	if !jsonout {
		fmt.Println(green("OK"))
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
	args := map[string]string{"<filenames>": "One or more directories or files to upload and update."}
	var uploadCmd = &cobra.Command{
		Use:   "commit [<filenames>]...",
		Short: "Record changes in local repository",
		Long:  formatdesc(description, args),
		Args:  cobra.ArbitraryArgs,
		Run:   commit,
		DisableFlagsInUseLine: true,
	}
	uploadCmd.Flags().Bool("json", false, "Print output in JSON format.")
	return uploadCmd
}
