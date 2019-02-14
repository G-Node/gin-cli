package gincmd

import (
	"fmt"
	"os"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func download(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	verbose, _ := cmd.Flags().GetBool("verbose")
	// TODO: no client necessary? Just use remotes
	conf := config.Read()
	gincl := ginclient.New(conf.DefaultServer)
	requirelogin(cmd, gincl, !jsonout)
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}

	content, _ := cmd.Flags().GetBool("content")
	lockchan := make(chan git.RepoFileStatus)
	if verbose {
		fmt.Printf("Running Gin Command: %v \n", cmd.Name())
	}
	go gincl.LockContent([]string{}, lockchan)
	formatOutput(lockchan, 0, jsonout, verbose)
	if !jsonout && !verbose {
		fmt.Print("Downloading changes ")
	}
	err := gincl.Download()
	CheckError(err)
	if !jsonout && !verbose {
		fmt.Fprintln(color.Output, green("OK"))
	}
	if content {
		reporoot, _ := git.FindRepoRoot(".")
		os.Chdir(reporoot)
		getContent(cmd, nil)
	}
}

// DownloadCmd sets up the 'download' subcommand
func DownloadCmd() *cobra.Command {
	description := "Downloads changes from the remote repository to the local clone. This will create new files that were added remotely, delete files that were removed, and update files that were changed.\n\nOptionally downloads the content of all files in the repository. If 'content' is not specified, new files will be empty placeholders. Content of individual files can later be retrieved using the 'get-content' command."
	var cmd = &cobra.Command{
		Use:                   "download [--json | --verbose] [--content]",
		Short:                 "Download all new information from a remote repository",
		Long:                  formatdesc(description, nil),
		Args:                  cobra.NoArgs,
		Run:                   download,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("json", false, "Print output in JSON format.")
	cmd.Flags().Bool("verbose", false, "Print all raw information.")
	cmd.Flags().Bool("content", false, "Download the content for all files in the repository.")
	return cmd
}
