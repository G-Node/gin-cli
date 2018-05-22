package gincmd

import (
	"fmt"
	"strings"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func splitAliasRemote(remote string) (string, string) {
	// split on first colon and check if it's a known alias
	parts := strings.SplitN(remote, ":", 2)
	if len(parts) < 2 {
		Die("remote location must be of the form <server>:<repositoryname>, or <alias>:<repositoryname> (see \"gin help add-remote\")")
	}
	return parts[0], parts[1]
}

func parseRemote(remote string) string {
	alias, repopath := splitAliasRemote(remote)
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
}

func createRemote(cmd *cobra.Command, remote string) {
	conf := config.Read()
	gincl := ginclient.New(conf.GinHost)
	requirelogin(cmd, gincl, true)
	_, repopath := splitAliasRemote(remote)
	repopathParts := strings.SplitN(repopath, "/", 2)
	reponame := repopathParts[1]
	fmt.Printf(":: Creating repository '%s' ", repopath)
	err := gincl.CreateRepo(reponame, "")
	CheckError(err)
	fmt.Fprintln(color.Output, green("OK"))
}

func promptCreate(cmd *cobra.Command, remote string) {
	var response string
	fmt.Printf("Remote %s does not exist. Would you like to create it?\n", remote)
	for {
		fmt.Printf("[c]reate / [a]dd anyway / a[b]ort: ")
		fmt.Scanln(&response)

		switch strings.ToLower(response) {
		case "c", "create":
			createRemote(cmd, remote)
			return
		case "a", "add", "add anyway":
			return
		case "b", "abort":
			Exit("aborted")
		}
	}
}

func addRemote(cmd *cobra.Command, args []string) {
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}
	flags := cmd.Flags()
	create, _ := flags.GetBool("create")
	name, remote := args[0], args[1]
	url := parseRemote(remote)
	err := checkRemote(cmd, url)
	// TODO: Check if it's a gin URL before offering to create
	if err != nil {
		if create {
			createRemote(cmd, remote)
		} else {
			promptCreate(cmd, remote)
		}
	}
	err = git.AddRemote(name, url)
	CheckError(err)
	fmt.Printf(":: Added new remote: %s [%s]\n", name, url)
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
	addRemoteCmd.Flags().Bool("create", false, "Create the remote on the server if it does not already exist.")
	return addRemoteCmd
}
