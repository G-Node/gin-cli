package gincmd

import (
	"github.com/spf13/cobra"
)

func addRemote(cmd *cobra.Command, args []string) {
	Die("Not implemented")
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
