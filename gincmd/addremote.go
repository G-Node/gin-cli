package gincmd

import (
	"fmt"
	"strings"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/git"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func parseRemote(remote string) string {
	// split on first colon and check if it's a known alias
	parts := strings.SplitN(remote, ":", 2)
	if len(parts) < 2 {
		Die("remote location must be of the form <server>:<repositoryname>, or <alias>:<repositoryname> (see \"gin help add-remote\")")
	}
	alias, repopath := parts[0], parts[1]
	if alias == "gin" {
		// Built-in alias 'gin'; use default remote address
		conf := config.Read()
		url := fmt.Sprintf("ssh://%s@%s/%s", conf.GitUser, conf.GitHost, repopath)
		return url
	}
	// Unknown alias, return as is
	return remote
}

func checkRemote(cmd *cobra.Command, url string) (err error) {
	// Check if the remote is accessible
	fmt.Print(":: Checking remote ")
	if _, err = git.LsRemote(url); err == nil {
		fmt.Fprintln(color.Output, green("OK"))
		return nil
	}
	fmt.Fprintln(color.Output, red("FAILED"))
	return err

	// TODO: Check if it's a gin URL before offering to create
	// conf := config.Read()
	// gincl := ginclient.New(conf.GinHost)
	// requirelogin(cmd, gincl, true)
	// Check again
}

func addRemote(cmd *cobra.Command, args []string) {
	if !git.IsRepo() {
		Die("This command must be run from inside a gin repository.")
	}
	name, remote := args[0], args[1]
	url := parseRemote(remote)
	err := git.AddRemote(name, url)
	CheckError(err)
	fmt.Printf(":: Added new remote: %s [%s]\n", name, url)
	err = checkRemote(cmd, url)
	if err != nil {
		// Prompt for cleanup
		git.Command("remote", "rm", name).Run()
		Die(err)
	}
	// CheckError(err)
}

// AddRemoteCmd sets up the 'add-remote' repository subcommand
func AddRemoteCmd() *cobra.Command {
	description := "Adds a server or location for data storage."
	args := map[string]string{
		"<name>":     "The name of the new remote",
		"<location>": "The server address or location for the data store.",
	}
	examples := map[string]string{
		"Add a GIN server repository as a remote": "$ …",
		"Add a storage drive":                     "$ …",
	}
	var addRemoteCmd = &cobra.Command{
		Use:     "add-remote <name> <location>",
		Short:   description,
		Long:    formatdesc(description, args),
		Example: formatexamples(examples),
		Args:    cobra.ExactArgs(2),
		Run:     addRemote,
		DisableFlagsInUseLine: true,
	}
	return addRemoteCmd
}
