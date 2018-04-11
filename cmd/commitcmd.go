package gincmd

import (
	"fmt"
	"os"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func commit(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, !jsonout)
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser

	lockchan := make(chan ginclient.RepoFileStatus)
	go gincl.LockContent(args, lockchan)
	formatOutput(lockchan, jsonout)

	// add header commit line
	hostname, err := os.Hostname()
	if err != nil {
		util.LogWrite("Could not retrieve hostname")
		hostname = "(unknown)"
	}
	commitmsg := fmt.Sprintf("gin upload from %s\n\n%s", hostname, getchanges())
	uploadchan := make(chan ginclient.RepoFileStatus)
	go gincl.Commit(args, commitmsg, uploadchan)
	formatOutput(uploadchan, jsonout)
}

func getchanges() string {
	// TODO FIXME: This function will often return "No changes" when changes are clearly made
	changes, err := ginclient.DescribeIndexShort()
	if err != nil {
		util.LogWrite("Failed to determine file changes for commit message")
	}
	if changes == "" {
		changes = "No changes recorded"
	}
	return changes
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
