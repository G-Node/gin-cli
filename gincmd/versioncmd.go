package gincmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/git"
	"github.com/G-Node/gin-cli/util"
	"github.com/spf13/cobra"
)

func repoversion(cmd *cobra.Command, args []string) {
	if !git.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	count, _ := cmd.Flags().GetUint("max-count")
	jsonout, _ := cmd.Flags().GetBool("json")
	commithash, _ := cmd.Flags().GetString("id")
	copyto, _ := cmd.Flags().GetString("copy-to")
	paths := args

	var commit git.GinCommit
	if commithash == "" {
		commits, err := git.Log(count, "", paths, false)
		util.CheckError(err)
		if jsonout {
			j, _ := json.Marshal(commits)
			fmt.Println(string(j))
			return
		}
		if len(commits) == 0 {
			util.Die("No revisions matched request")
		}
		commit = verprompt(commits)
	} else {
		commits, err := git.Log(1, commithash, paths, false)
		util.CheckError(err)
		commit = commits[0]
	}

	if copyto == "" {
		// TODO: Print some sort of output (similar to copy-to variant)
		// e.g., File 'fname' restored to version <revision> (date)
		err := ginclient.CheckoutVersion(commit.AbbreviatedHash, paths)
		util.CheckError(err)

		addchan := make(chan git.RepoFileStatus)
		go ginclient.Add(paths, addchan)
		formatOutput(addchan, jsonout)

		fmt.Print("Recording changes ")
		err = git.Commit(makeCommitMessage("commit", paths))
		if err != nil {
			util.Die(err)
		}
		fmt.Println(green("OK"))

	} else {
		checkoutcopies(commit, paths, copyto)
	}
}

func checkoutcopies(commit git.GinCommit, paths []string, destination string) {
	hash := commit.AbbreviatedHash
	isodate := commit.Date.Format("2006-01-02-1504")
	prettydate := commit.Date.Format("Jan 2 15:04:05 2006 (-0700)")
	checkoutchan := make(chan ginclient.FileCheckoutStatus)
	go ginclient.CheckoutFileCopies(hash, paths, destination, isodate, checkoutchan)

	// TODO: JSON output
	var newfiles []string
	for costatus := range checkoutchan {
		if costatus.Err != nil {
			fmt.Println(costatus.Err.Error())
			continue
		}
		switch costatus.Type {
		case "Git":
			fmt.Printf("Copied git file '%s' from revision %s (%s) to '%s'\n", costatus.Filename, hash, prettydate, costatus.Destination)
			newfiles = append(newfiles, costatus.Destination)
		case "Annex":
			fmt.Printf("Copied placeholder file '%s' from revision %s (%s) to '%s'\n", costatus.Filename, hash, prettydate, costatus.Destination)
			// TODO: Check if contents are available locally and if not advise with 'gin get-content' command
		case "Link":
			fmt.Printf("'%s' is a link to '%s' and it not a placeholder file; cannot recover\n", costatus.Filename, costatus.Destination)
		}
	}
	// Add new files to index but do not upload
	addchan := make(chan git.RepoFileStatus)
	go git.Add(newfiles, addchan)
	<-addchan
	// TODO: Instead of adding git files, would it be better if we did get-content on annex files and then removed them from the index?
}

func verprompt(commits []git.GinCommit) git.GinCommit {
	ndigits := len(strconv.Itoa(len(commits) + 1))
	numfmt := fmt.Sprintf("[%%%dd]", ndigits)
	width := termwidth()
	for idx, commit := range commits {
		idxstr := fmt.Sprintf(numfmt, idx+1)
		fmt.Printf("%s  %s * %s\n\n", idxstr, green(commit.AbbreviatedHash), commit.Date.Format("Mon Jan 2 15:04:05 2006 (-0700)"))
		fmt.Printf("%s\n", winner.Wrap(commit.Subject, width))
		if len(commit.Body) > 0 {
			fmt.Printf("%s\n", winner.Wrap(commit.Body, width))
		}
		fstats := commit.FileStats
		if len(fstats.NewFiles) > 0 {
			fmt.Printf("  Added\n%s\n", winner.Wrap(strings.Join(fstats.NewFiles, ", "), width))
		}
		if len(fstats.ModifiedFiles) > 0 {
			fmt.Printf("  Modified\n%s\n", winner.Wrap(strings.Join(fstats.ModifiedFiles, ", "), width))
		}
		if len(fstats.DeletedFiles) > 0 {
			fmt.Printf("  Deleted\n%s\n", winner.Wrap(strings.Join(fstats.DeletedFiles, ", "), width))
		}
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
	return git.GinCommit{}
}

// VersionCmd sets up the 'version' subcommand
func VersionCmd() *cobra.Command {
	description := "Roll back directories or files to older versions."
	args := map[string]string{"<filenames>": "One or more directories or files to roll back."}
	examples := map[string]string{
		"Show the 50 most recent versions of recordings.nix and prompt for version":                                                "$ gin version -n 50 recordings.nix",
		"Return the files in the code/ directory to the version with ID 429d51e":                                                   "$ gin version --id 429d51e code/",
		"Retrieve all files from the code/ directory from version with ID 918a06f and copy it to a directory called oldcode/":      "$ gin version --id 918a06f --copy-to oldcode code",
		"Show the 15 most recent versions of data.zip, prompt for version, and copy the selected version to the current directory": "$ gin version -n 15 --copy-to . data.zip",
	}
	var versionCmd = &cobra.Command{
		Use:     "version [--json] [--max-count n | --id hash | --copy-to location] [<filenames>]...",
		Short:   "Roll back files or directories to older versions",
		Long:    formatdesc(description, args),
		Example: formatexamples(examples),
		Args:    cobra.ArbitraryArgs,
		Run:     repoversion,
		DisableFlagsInUseLine: true,
	}
	versionCmd.Flags().Bool("json", false, "Print output in JSON format.")
	versionCmd.Flags().UintP("max-count", "n", 10, "Maximum `number` of versions to display before prompting. 0 means 'all'.")
	versionCmd.Flags().String("id", "", "Commit `ID` (hash) to return to.")
	versionCmd.Flags().String("copy-to", "", "Retrieve files from history and copy them to a new `location` instead of overwriting the existing ones. The new files will be placed in the directory specified and will be renamed to include the date and time of their version.")
	return versionCmd
}
