package gincmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func lsRepo(cmd *cobra.Command, args []string) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	flags := cmd.Flags()
	if flags.NFlag() > 1 {
		usageDie(cmd)
	}
	jsonout, _ := flags.GetBool("json")
	short, _ := flags.GetBool("short")

	gincl := ginclient.NewClient(util.Config.GinHost)
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser

	filesStatus, err := gincl.ListFiles(args...)
	util.CheckError(err)

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
		util.CheckError(err)
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

func LsRepoCmd() *cobra.Command {
	var lsRepoCmd = &cobra.Command{
		Use:   "ls [--json | --short | -s] [<filenames>]...",
		Short: "List the sync status of files in the local repository",
		Long:  "List one or more files or the contents of directories and their status. With no arguments, lists the status of the files under the current directory. Directory listings are performed recursively.",
		Args:  cobra.ArbitraryArgs,
		Run:   lsRepo,
		DisableFlagsInUseLine: true,
	}
	lsRepoCmd.Flags().Bool("json", false, "Print listing in JSON format (uses short form abbreviations).")
	lsRepoCmd.Flags().BoolP("short", "s", false, "Print listing in short form.")
	return lsRepoCmd
}
