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
	commithash, _ := cmd.Flags().GetString("id")
	copyto, _ := cmd.Flags().GetString("copy-to")
	paths := args

	var commit ginclient.GinCommit
	if commithash == "" {
		commits, err := ginclient.GitLog(count, "", paths, false)
		util.CheckError(err)
		if jsonout {
			j, _ := json.Marshal(commits)
			fmt.Println(string(j))
			return
		}
		commit = verprompt(commits)
	} else {
		commits, err := ginclient.GitLog(1, commithash, paths, false)
		util.CheckError(err)
		commit = commits[0]
	}

	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, true) // TODO: change when we support offline-only
	var err error
	if copyto == "" {
		err = ginclient.CheckoutVersion(commit.AbbreviatedHash, paths)
		hostname, herr := os.Hostname()
		if herr != nil {
			util.LogWrite("Could not retrieve hostname")
			hostname = "(unknown)"
		}

		commitsubject := fmt.Sprintf("Repository version changed by %s@%s", gincl.Username, hostname)
		commitbody := fmt.Sprintf("Returning to version as of %s\nVersion ID: %s\n%s", commit.Date.Format("Mon Jan 2 15:04:05 2006 (-0700)"), commit.AbbreviatedHash, getchanges())
		commitmsg := fmt.Sprintf("%s\n\n%s", commitsubject, commitbody)

		uploadchan := make(chan ginclient.RepoFileStatus)
		go gincl.Upload(paths, commitmsg, uploadchan)
		printProgress(uploadchan, jsonout)
	} else {
		err = ginclient.CheckoutFileCopies(commit.AbbreviatedHash, paths, copyto)
	}
	util.CheckError(err)
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
	fmt.Print("Version to retrieve files from: ")
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

// VersionCmd sets up the 'version' subcommand
func VersionCmd() *cobra.Command {
	description := "Roll back directories or files to older versions. See also the 'copy-version' command."
	args := map[string]string{"<filenames>": "One or more directories or files to roll back."}
	examples := map[string]string{
		"Show the 50 most recent versions of recordings.nix and prompt for version":                                                "$ gin version -n 50 recordings.nix",
		"Return the files in the code/ directory to the version with ID 429d51e":                                                   "$ gin version --id 429d51e code/",
		"Retrieve all files from the code/ directory from version with ID 918a06f and copy it to a directory called oldcode/":      "$ gin version --id 918a06f --copy-to oldcode code",
		"Show the 15 most recent versions of data.zip, prompt for version, and copy the selected version to the current directory": "$ gin version -n 15 --copy-to . data.zip",
	}
	var versionCmd = &cobra.Command{
		Use:     "version [--json] [--max-count n | --id hash] [<filenames>]...",
		Short:   "Roll back files or directories to older versions",
		Long:    formatdesc(description, args),
		Example: formatexamples(examples),
		Args:    cobra.ArbitraryArgs,
		Run:     repoversion,
		DisableFlagsInUseLine: true,
	}
	versionCmd.Flags().Bool("json", false, "Print output in JSON format.")
	versionCmd.Flags().UintP("max-count", "n", 10, "Maximum number of versions to display before prompting. 0 means 'all'.")
	versionCmd.Flags().String("id", "", "Commit ID (hash) to return to.")
	versionCmd.Flags().String("copy-to", "", "Retrieve files from history and copy them to a new location instead of overwriting the existing ones. The new files will be placed in the directory specified and will be renamed to include the date and time of their version.")
	return versionCmd
}
