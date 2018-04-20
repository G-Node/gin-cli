package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func upload(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	gincl := ginclient.New(config.Config.GinHost)
	requirelogin(cmd, gincl, !jsonout)
	if !git.IsRepo() {
		Die("This command must be run from inside a gin repository.")
	}

	gincl.GitHost = config.Config.GitHost
	gincl.GitUser = config.Config.GitUser

	paths := args

	if len(paths) > 0 {
		// Don't add + commit files if nothing was specified
		addchan := make(chan git.RepoFileStatus)
		go ginclient.Add(paths, addchan)
		formatOutput(addchan, jsonout)

		fmt.Print("Recording changes ")
		// ignore error for now :: call commit() instead
		git.Commit(makeCommitMessage("upload", paths))
		fmt.Println(green("OK"))
	}

	uploadchan := make(chan git.RepoFileStatus)
	go gincl.Upload(paths, uploadchan)
	formatOutput(uploadchan, jsonout)
}

// UploadCmd sets up the 'upload' subcommand
func UploadCmd() *cobra.Command {
	description := "Upload changes made in a local repository clone to the remote repository on the GIN server. This command must be called from within the local repository clone. Specific files or directories may be specified. All changes made will be sent to the server, including addition of new files, modifications and renaming of existing files, and file deletions.\n\nIf no arguments are specified, only changes to files already being tracked are uploaded."
	args := map[string]string{"<filenames>": "One or more directories or files to upload and update."}
	var uploadCmd = &cobra.Command{
		Use:   "upload [--json] [<filenames>]...",
		Short: "Upload local changes to a remote repository",
		Long:  formatdesc(description, args),
		Args:  cobra.ArbitraryArgs,
		Run:   upload,
		DisableFlagsInUseLine: true,
	}
	uploadCmd.Flags().Bool("json", false, "Print output in JSON format.")
	return uploadCmd
}
