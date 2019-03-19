package gincmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func lsRepo(cmd *cobra.Command, args []string) {
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}

	flags := cmd.Flags()
	if flags.NFlag() > 1 {
		usageDie(cmd)
	}
	jsonout, _ := flags.GetBool("json")
	short, _ := flags.GetBool("short")

	// TODO: Use repo remotes; no server configuration
	gincl := ginclient.New("gin")

	filesStatus, err := gincl.ListFiles(args...)
	CheckError(err)

	// TODO: Print warning when in direct mode: git files that have not been uploaded will show up as synced.

	if short {
		for fname, status := range filesStatus {
			fmt.Printf("%s %s\n", status.Abbrev(), fname)
		}
	} else if jsonout {
		type fstat struct {
			FileName string `json:"filename"`
			Status   string `json:"status"`
		}
		var statuses []fstat
		for fname, status := range filesStatus {
			statuses = append(statuses, fstat{FileName: fname, Status: status.Abbrev()})
		}
		jsonbytes, err := json.Marshal(statuses)
		CheckError(err)
		fmt.Println(string(jsonbytes))
	} else {
		// Files are printed separated by status and sorted by name
		statFiles := make(map[ginclient.FileStatus][]string)

		for file, status := range filesStatus {
			statFiles[status] = append(statFiles[status], file)
		}

		// sort files in each status (stable sorting unnecessary)
		// also collect active statuses for sorting
		var statuses ginclient.FileStatusSlice
		for status := range statFiles {
			sort.Sort(sort.StringSlice(statFiles[status]))
			statuses = append(statuses, status)
		}
		sort.Sort(statuses)

		// print each category with len(items) > 0 with appropriate header
		for _, status := range statuses {
			fmt.Printf("%s:\n", status.Description())
			fmt.Printf("\n\t%s\n\n", strings.Join(statFiles[status], "\n\t"))
		}
	}
}

// LsRepoCmd sets up the file 'ls' subcommand
func LsRepoCmd() *cobra.Command {

	description := `List one or more files or the contents of directories and the status of the files within it. With no arguments, lists the status of the files under the current directory. Directory listings are performed recursively.

In the short form, the meaning of the status abbreviations is as follows:
OK: The file is part of the GIN repository and its contents are synchronised with the server.
TC: The file has been locked or unlocked and the change has not been recorded yet (and it is unmodified).
NC: The local file is a placeholder and its contents have not been downloaded.
MD: The file has been modified locally and the changes have not been recorded yet.
LC: The file has been modified locally, the changes have been recorded but they haven't been uploaded.
RM: The file has been removed from the repository.
??: The file is not under repository control.`

	args := map[string]string{
		"<filenames>": "One or more directories or files to list.",
	}

	var cmd = &cobra.Command{
		Use:                   "ls [--json | --short | -s] [<filenames>]...",
		Short:                 "List the sync status of files in the local repository",
		Long:                  formatdesc(description, args),
		Args:                  cobra.ArbitraryArgs,
		Run:                   lsRepo,
		Aliases:               []string{"status"},
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, "Print listing in JSON format (uses short form abbreviations).")
	cmd.Flags().BoolP("short", "s", false, "Print listing in short form.")
	return cmd
}
