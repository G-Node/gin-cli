package gincmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func repoversion(cmd *cobra.Command, args []string) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	count, _ := cmd.Flags().GetUint("max-count")
	jsonout, _ := cmd.Flags().GetBool("json")
	paths := args
	commits, err := ginclient.GitLog(count, paths)
	util.CheckError(err)
	if jsonout {
		j, _ := json.Marshal(commits)
		fmt.Println(string(j))
		return
	}
	commit := verprompt(commits)

	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, true) // TODO: change when we support offline-only
	checkout(commit, gincl)
}

func verprompt(commits []ginclient.GinCommit) ginclient.GinCommit {
	ndigits := len(strconv.Itoa(len(commits) + 1))
	numfmt := fmt.Sprintf("[%%%dd]", ndigits)
	for idx, commit := range commits {
		idxstr := fmt.Sprintf(numfmt, idx+1)
		fmt.Printf("%s  %s * %s\n\n", idxstr, green(commit.AbbreviatedHash), commit.Date.Format("Mon Jan 2 15:04:05 2006 (-0700)"))
		fmt.Printf("\t%s\n\n", commit.Subject)
		fstats := commit.FileStats
		// TODO: wrap file listings
		if len(fstats.NewFiles) > 0 {
			fmt.Printf("\tAdded:    %s\n", strings.Join(fstats.NewFiles, ", "))
		}
		if len(fstats.ModifiedFiles) > 0 {
			fmt.Printf("\tModified: %s\n", strings.Join(fstats.ModifiedFiles, ", "))
		}
		if len(fstats.DeletedFiles) > 0 {
			fmt.Printf("\tDeleted:  %s\n", strings.Join(fstats.DeletedFiles, ", "))
		}
		fmt.Println()
	}
	var selstr string
	fmt.Print("Version to change to: ")
	fmt.Scanln(&selstr)

	num, err := strconv.Atoi(selstr)
	if err == nil && num > 0 && num <= len(commits) {
		return commits[num-1]
	}

	// try to match hash
	for _, commit := range commits {
		if commit.AbbreviatedHash == selstr {
			return commit
		}
	}

	util.Die("Aborting")
	return ginclient.GinCommit{}
}

func checkout(commit ginclient.GinCommit, gincl *ginclient.Client) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	ginclient.GitCheckout(commit.Hash, nil)
	uploadchan := make(chan ginclient.RepoFileStatus) // TODO: change to commit when we support offline-only
	hostname, err := os.Hostname()
	if err != nil {
		util.LogWrite("Could not retrieve hostname")
		hostname = "(unknown)"
	}

	commitsubject := fmt.Sprintf("Repository version changed by %s@%s", gincl.Username, hostname)
	commitbody := fmt.Sprintf("Returning to version as of %s\nVersion ID: %s\n%s", commit.Date.Format("Mon Jan 2 15:04:05 2006 (-0700)"), commit.AbbreviatedHash, getchanges())
	commitmsg := fmt.Sprintf("%s\n\n%s", commitsubject, commitbody)

	go gincl.Upload([]string{}, commitmsg, uploadchan)
	printProgress(uploadchan, false)
}

// VersionCmd sets up the 'version' subcommand
func VersionCmd() *cobra.Command {
	description := "Roll back directories or files to older versions."
	args := map[string]string{"<filenames>": "One or more directories or files to roll back."}
	examples := map[string]string{"Example 1": "$ gin version -n 50"}
	var versionCmd = &cobra.Command{
		Use:     "version [--json] [--max-count] [<filenames>]...",
		Short:   "Roll back files or directories to older versions",
		Long:    formatdesc(description, args),
		Example: formatexamples(examples),
		Args:    cobra.ArbitraryArgs,
		Run:     repoversion,
		DisableFlagsInUseLine: true,
	}
	versionCmd.Flags().Bool("json", false, "Print output in JSON format.")
	versionCmd.Flags().UintP("max-count", "n", 10, "Maximum number of versions to display before prompting. 0 means 'all'.")
	return versionCmd
}
