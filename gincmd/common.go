package gincmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/git"
	"github.com/G-Node/gin-cli/util"
	"github.com/bbrks/wrap"
	"github.com/docker/docker/pkg/term"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

const unknownhostname = "(unknown)"

var green = color.New(color.FgGreen).SprintFunc()
var red = color.New(color.FgRed).SprintFunc()

// requirelogin prompts for login if the user is not already logged in.
// It only checks if a local token exists and does not confirm its validity with the server.
// The function should be called at the start of any command that requires being logged in to run.
func requirelogin(cmd *cobra.Command, gincl *ginclient.Client, prompt bool) {
	err := gincl.LoadToken()
	if prompt {
		if err != nil {
			login(cmd, nil)
		}
		err = gincl.LoadToken()
	}
	util.CheckError(err)
}

func usageDie(cmd *cobra.Command) {
	cmd.Help()
	// exit without message
	util.Die("")
}

func printJSON(statuschan <-chan git.RepoFileStatus) (filesuccess map[string]bool) {
	filesuccess = make(map[string]bool)
	for stat := range statuschan {
		j, _ := json.Marshal(stat)
		fmt.Println(string(j))
		filesuccess[stat.FileName] = true
		if stat.Err != nil {
			filesuccess[stat.FileName] = false
		}
	}
	return
}

func printProgressOutput(statuschan <-chan git.RepoFileStatus) (filesuccess map[string]bool) {
	filesuccess = make(map[string]bool)
	var fname, state string
	var lastprint string
	outline := new(bytes.Buffer)
	outappend := func(part string) {
		if len(part) > 0 {
			outline.WriteString(part)
			outline.WriteString(" ")
		}
	}
	for stat := range statuschan {
		outline.Reset()
		if stat.FileName != fname || stat.State != state {
			// New line if new file or new state
			if len(lastprint) > 0 {
				fmt.Println()
			}
			lastprint = ""
			fname = stat.FileName
			state = stat.State
		}
		outappend(stat.State)
		outappend(stat.FileName)
		if stat.Err == nil {
			if stat.Progress == "100%" {
				outappend(green("OK"))
				filesuccess[stat.FileName] = true
			} else {
				outappend(stat.Progress)
				outappend(stat.Rate)
			}
		} else {
			outappend(stat.Err.Error())
			filesuccess[stat.FileName] = false
		}
		newprint := outline.String()
		if newprint != lastprint {
			fmt.Printf("\r%s\r", strings.Repeat(" ", len(lastprint))) // clear the line
			fmt.Fprint(color.Output, newprint)
			lastprint = newprint
		}
	}
	if len(lastprint) > 0 {
		fmt.Println()
	}
	return
}

func formatOutput(statuschan <-chan git.RepoFileStatus, jsonout bool) {
	var filesuccess map[string]bool
	if jsonout {
		filesuccess = printJSON(statuschan)
	} else {
		filesuccess = printProgressOutput(statuschan)
	}

	// count unique file errors
	nerrors := 0
	for _, stat := range filesuccess {
		if !stat {
			nerrors++
		}
	}
	if nerrors > 0 {
		// Exit with error message and failed exit status
		var plural string
		if nerrors > 1 {
			plural = "s"
		}
		util.Die(fmt.Sprintf("%d operation%s failed", nerrors, plural))
	}
}

var wouter = wrap.NewWrapper()
var winner = wrap.NewWrapper()

func termwidth() int {
	width := 80
	if ws, err := term.GetWinsize(0); err == nil {
		width = int(ws.Width)
	}
	return width - 1
}

func formatdesc(desc string, args map[string]string) (fdescription string) {
	width := termwidth()
	wouter.OutputLinePrefix = "  "
	winner.OutputLinePrefix = "    "

	if len(desc) > 0 {
		fdescription = fmt.Sprintf("Description:\n\n%s", wouter.Wrap(desc, width))
	}

	if args != nil {
		argsdesc := fmt.Sprintf("Arguments:\n\n")
		for a, d := range args {
			argsdesc = fmt.Sprintf("%s%s%s\n", argsdesc, wouter.Wrap(a, width), winner.Wrap(d, width))
		}
		fdescription = fmt.Sprintf("%s\n%s", fdescription, argsdesc)
	}
	return
}

func formatexamples(examples map[string]string) (exdesc string) {
	width := termwidth()
	if examples != nil {
		for d, ex := range examples {
			exdesc = fmt.Sprintf("%s\n%s%s", exdesc, wouter.Wrap(d, width), winner.Wrap(ex, width))
		}
	}
	return
}

// SetUpCommands sets up all the subcommands for the client and returns the root command, ready to execute.
func SetUpCommands(verstr string) *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:                   "gin",
		Long:                  "GIN Command Line Interface and client for the GIN services", // TODO: Add license and web info
		Version:               fmt.Sprintln(verstr),
		DisableFlagsInUseLine: true,
	}
	cobra.AddTemplateFunc("wrappedFlagUsages", wrappedFlagUsages)
	rootCmd.SetHelpTemplate(helpTemplate)
	rootCmd.SetUsageTemplate(usageTemplate)

	// Login
	rootCmd.AddCommand(LoginCmd())

	// Logout
	rootCmd.AddCommand(LogoutCmd())

	// Init repo
	rootCmd.AddCommand(InitCmd())

	// Create repo
	rootCmd.AddCommand(CreateCmd())

	// Delete repo (unlisted)
	rootCmd.AddCommand(DeleteCmd())

	// Get repo
	rootCmd.AddCommand(GetCmd())

	// List files
	rootCmd.AddCommand(LsRepoCmd())

	// Unlock content
	rootCmd.AddCommand(UnlockCmd())

	// Lock content
	rootCmd.AddCommand(LockCmd())

	// Commit changes
	rootCmd.AddCommand(CommitCmd())

	// Upload
	rootCmd.AddCommand(UploadCmd())

	// Download
	rootCmd.AddCommand(DownloadCmd())

	// Get content
	rootCmd.AddCommand(GetContentCmd())

	// Remove content
	rootCmd.AddCommand(RemoveContentCmd())

	// Account info
	rootCmd.AddCommand(InfoCmd())

	// List repos
	rootCmd.AddCommand(ReposCmd())

	// Repo info
	rootCmd.AddCommand(RepoInfoCmd())

	// Keys
	rootCmd.AddCommand(KeysCmd())

	// Version
	rootCmd.AddCommand(VersionCmd())

	// git and annex passthrough (unlisted)
	rootCmd.AddCommand(GitCmd())
	rootCmd.AddCommand(AnnexCmd())

	return rootCmd
}
