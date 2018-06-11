package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func upload(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	remotes, _ := cmd.Flags().GetStringSlice("to")
	gincl := ginclient.New("gin") // TODO: probably doesn't need a client
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}

	// Fail early if no default remote
	if _, err := ginclient.DefaultRemote(); err != nil && len(remotes) == 0 {
		Die("upload failed: no remote configured")
	}

	// If any of the specified remotes is the special name 'all', upload to all configured remotes
	for _, remote := range remotes {
		if remote == "all" {
			confremotes, err := git.RemoteShow()
			CheckErrorMsg(err, fmt.Sprintf("'all' remotes specified, but could not determine configured remotes: %s", err))
			remotes = make([]string, 0, len(confremotes))
			for r := range confremotes {
				remotes = append(remotes, r)
			}
			break
		}
	}

	paths := args
	if len(paths) > 0 {
		commit(cmd, paths)
	}

	uploadchan := make(chan git.RepoFileStatus)
	go gincl.Upload(paths, remotes, uploadchan)
	formatOutput(uploadchan, 0, jsonout)
}

// UploadCmd sets up the 'upload' subcommand
func UploadCmd() *cobra.Command {
	description := `Upload changes made in a local repository clone to the remote repository on the GIN server. This command must be called from within the local repository clone. Specific files or directories may be specified. All changes made will be sent to the server, including addition of new files, modifications and renaming of existing files, and file deletions.

You can specify which remotes the content will be uploaded to using the --to flag. The flag can be specified multiple times. If the keyword 'all' is specified as a remote, the data is uploaded to all configured remotes.

If no arguments are specified, only changes to files already being tracked are uploaded.`

	args := map[string]string{"<filenames>": "One or more directories or files to upload and update."}
	examples := map[string]string{
		"Upload 'data1.dat' and 'values.csv' to default remote":             "$ gin upload data1.dat values.csv",
		"Upload all files in current directory to default remote":           "$ gin upload .",
		"Upload all previously committed changes to remote named 'labdata'": "$ gin upload --to labdata",
		"Upload all '.zip' files to remotes named 'gin' and 'labdata'":      "$ gin upload --to gin --to labdata *.zip\n    or\n$ gin upload --to gin,labdata *.zip",
	}
	var cmd = &cobra.Command{
		Use:     "upload [--json] [--to <remote>] [<filenames>]...",
		Short:   "Upload local changes to a remote repository",
		Long:    formatdesc(description, args),
		Args:    cobra.ArbitraryArgs,
		Example: formatexamples(examples),
		Run:     upload,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, "Print output in JSON format.")
	cmd.Flags().StringSliceP("to", "t", nil, "Upload to specific `remote`. Supports multiple remotes, either by specifying multiple times or as a comma separated list (see Examples). If the keyword 'all' is specified, the data is uploaded to all configured remotes.")
	return cmd
}
