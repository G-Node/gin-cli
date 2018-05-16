package gincmd

import (
	"fmt"
	"strings"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/git"
	"github.com/spf13/cobra"
)

func parseRemote(remote string) string {
	// split on first colon and check if it's a known alias
	parts := strings.SplitN(remote, ":", 2)
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

func addRemote(cmd *cobra.Command, args []string) {
	if !git.IsRepo() {
		Die("This command must be run from inside a gin repository.")
	}
	name, remote := args[0], args[1]
	url := parseRemote(remote)
	git.AddRemote(name, url)

	// TODO: Check if remote exists (and is accessible)

	// TODO: If it doesn't exist, offer to create it
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
