package gincmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func repoversion(cmd *cobra.Command, args []string) {
	switch git.Checkwd() {
	case git.NotRepository:
		Die(ginerrors.NotInRepo)
	case git.NotAnnex:
		Warn(ginerrors.MissingAnnex)
	case git.UpgradeRequired:
		annexVersionNotice()
	}
	count, _ := cmd.Flags().GetUint("max-count")
	jsonout, _ := cmd.Flags().GetBool("json")
	commithash, _ := cmd.Flags().GetString("id")
	copyto, _ := cmd.Flags().GetString("copy-to")
	paths := args

	var gcommit git.GinCommit
	if commithash == "" {
		commits, err := git.Log(count, "", paths, false)
		CheckError(err)
		if jsonout {
			j, _ := json.Marshal(commits)
			fmt.Println(string(j))
			return
		}
		if len(commits) == 0 {
			Die("No revisions matched request")
		}
		gcommit = verprompt(commits)
	} else {
		commits, err := git.Log(1, commithash, paths, false)
		CheckError(err)
		gcommit = commits[0]
	}

	if copyto == "" {
		// TODO: Print some sort of output (similar to copy-to variant)
		// e.g., File 'fname' restored to version <revision> (date)
		err := ginclient.CheckoutVersion(gcommit.AbbreviatedHash, paths)
		CheckError(err)
		commit(cmd, paths)
	} else {
		checkoutcopies(gcommit, paths, copyto)
	}
}

func checkoutcopies(commit git.GinCommit, paths []string, destination string) {
	hash := commit.AbbreviatedHash
	isodate := commit.Date.Format("2006-01-02-150405")
	prettydate := commit.Date.Format("Jan 2 15:04:05 2006 (-0700)")
	conf := config.Read()
	gincl := ginclient.New(conf.DefaultServer)
	checkoutchan := gincl.CheckoutFileCopies(hash, paths, destination, isodate)

	// TODO: JSON output
	var newfiles int
	var nerr int
	fmt.Println(":: Checking out old file versions")
	for costatus := range checkoutchan {
		if costatus.Err != nil {
			fmt.Printf("Failed to retrieve copy of '%s': %s\n", costatus.Filename, costatus.Err.Error())
			nerr++
			continue
		}
		switch costatus.Type {
		case "Git":
			fallthrough
		case "Annex":
			fallthrough
		case "Link":
			newfiles++
			fmt.Printf(" Copied file '%s' from revision %s (%s) to '%s'\n", costatus.Filename, hash, prettydate, costatus.Destination)
		case "Tree":
			fmt.Printf(" Created subdirectory '%s'\n", costatus.Destination)
		}
	}
	width := termwidth()
	wrapprint := func(fmtstr string, args ...interface{}) {
		fmt.Print(wouter.Wrap(fmt.Sprintf(fmtstr, args...), width))
	}
	fmt.Println()
	wrapprint("%d files were checked out from an older version", newfiles)
	if nerr > 0 {
		plural := ""
		if nerr > 1 {
			plural = "s"
		}
		Die(fmt.Sprintf("%d operation%s failed", nerr, plural))
	}
}

func verprompt(commits []git.GinCommit) git.GinCommit {
	ndigits := len(strconv.Itoa(len(commits) + 1))
	numfmt := fmt.Sprintf("[%%%dd]", ndigits)
	width := termwidth()
	for idx, commit := range commits {
		idxstr := fmt.Sprintf(numfmt, idx+1)
		fmt.Fprintf(color.Output, "%s  %s * %s\n\n", idxstr, green(commit.AbbreviatedHash), commit.Date.Format("Mon Jan 2 15:04:05 2006 (-0700)"))
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

	Die("Aborting")
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
	var cmd = &cobra.Command{
		Use:                   "version [--json] [--max-count n | --id hash | --copy-to location] [<filenames>]...",
		Short:                 "Roll back files or directories to older versions",
		Long:                  formatdesc(description, args),
		Example:               formatexamples(examples),
		Args:                  cobra.ArbitraryArgs,
		Run:                   repoversion,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, jsonHelpMsg)
	cmd.Flags().UintP("max-count", "n", 10, "Maximum `number` of versions to display before prompting. 0 means 'all'.")
	cmd.Flags().String("id", "", "Commit `ID` (hash) to return to.")
	cmd.Flags().String("copy-to", "", "Retrieve files from history and copy them to a new `location` instead of overwriting the existing ones. The new files will be placed in the directory specified and will be renamed to include the date and time of their version.")
	return cmd
}
