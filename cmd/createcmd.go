package gincmd

import (
	"fmt"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func createRepo(cmd *cobra.Command, args []string) {
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, true)

	var repoName, repoDesc string

	flags := cmd.Flags()
	here, _ := flags.GetBool("here")
	noclone, _ := flags.GetBool("no-clone")

	if noclone && here {
		usageDie(cmd)
	}

	if len(args) == 0 {
		fmt.Print("Repository name: ")
		fmt.Scanln(&repoName)
	} else {
		repoName = args[0]
		if len(args) == 2 {
			repoDesc = args[1]
		}
	}
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	repoPath := fmt.Sprintf("%s/%s", gincl.Username, repoName)
	fmt.Printf("Creating repository '%s' ", repoPath)
	err := gincl.CreateRepo(repoName, repoDesc)
	util.CheckError(err)
	fmt.Fprintln(color.Output, green("OK"))

	if here {
		// Init cwd
		ginclient.Workingdir = "."
		initchan := make(chan ginclient.RepoFileStatus)
		go gincl.InitDir(repoPath, initchan)
		formatOutput(initchan, false)
	} else if !noclone {
		// Clone repository after creation
		getRepo(cmd, []string{repoPath})
	}
}

// CreateCmd sets up the 'create' subcommand
func CreateCmd() *cobra.Command {
	description := "Create a new repository on the GIN server and optionally clone it locally or initialise working directory."

	args := map[string]string{
		"<name>":        "The name of the repository. If none is provided, you will be prompted for one. If you want to provide a description, you need to provide a repository name on the command line first and the description second. Names should contain only alphanumberic characters, '.', '-', and '_'.",
		"<description>": "A repository description (optional). The description should be specified as a single argument. For most shells, this means the description should be in quotes.",
	}

	examples := map[string]string{
		"Create a repository. Prompt for name":                                                               "$ gin create",
		"Create a repository named 'example' with no description":                                            "$ gin create example",
		"Create a repository named 'mydata' and initialise the current working directory as the local clone": "$ gin create --here mydata",
		"Create a repository named 'eegdata' with a description":                                             "$ gin create eegdata \"My repository for storing EEG data\"",
	}

	var createCmd = &cobra.Command{
		Use:     "create [--here | --no-clone] [<repository>] [<description>]",
		Short:   "Create a new repository on the GIN server",
		Long:    formatdesc(description, args),
		Example: formatexamples(examples),
		Args:    cobra.MaximumNArgs(2),
		Run:     createRepo,
		DisableFlagsInUseLine: true,
	}
	createCmd.Flags().Bool("here", false, "Create the local repository clone in the current working directory. Cannot be used with --no-clone.")
	createCmd.Flags().Bool("no-clone", false, "Create repository on the server but do not clone it locally. Cannot be used with --here.")
	return createCmd
}
