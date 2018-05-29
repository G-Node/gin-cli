package gincmd

import (
	"github.com/spf13/cobra"
)

func newRemote(cmd *cobra.Command, args []string) {
}

// NewRemoteCmd sets up the 'use-remote' repository subcommand
func NewRemoteCmd() *cobra.Command {
	description := `Configure a new GIN server that can be added as a remote to repositories.

The command requires only one argument, the alias for the server. All other information can be provided on the command line using the flags described below. You will be prompted for any required information that is not provided.

When configuring a server, you must specify an alias (name) for it, which will be used to refer to the configured server. This alias is then used when adding a remote to a repository. See 'gin help add-remote'.

The following information is required to configure a new server:

For the web server: the protocol, hostname, and port

    The protocol should be either 'http' or 'https'.
    The hostname for the server (e.g., web.gin.g-node.org).
    The web port for the server (e.g., 80, 443).

For the git server: the git user, hostname, and port

    The git user is the username set on the server for managing the repositories. It is almost always 'git'.
    The hostname for the git server (e.g., git.g-node.org).
    The git port for the server (e.g., 22, 2222).

See the Examples section for a full example.
`
	args := map[string]string{
		"<alias>": "The alias (name) for the server.",
	}
	examples := map[string]string{
		"This is what configuring the built-in GIN remote would look like (note: this is already configured)": "$ gin new-remote --web https://web.gin.g-node.org:443 --git git@git.g-node.org:22 gin",
	}
	var newRemoteCmd = &cobra.Command{
		Use:     "new-remote [--web http[s]://<hostname>[:<port>]] [--git [<gituser>@]<hostname>[:<port>]] <alias>",
		Short:   "Set the repository's default upload remote",
		Long:    formatdesc(description, args),
		Args:    cobra.MaximumNArgs(1),
		Example: formatexamples(examples),
		Run:     newRemote,
		DisableFlagsInUseLine: true,
	}
	newRemoteCmd.Flags().String("web", "", "Set the address and port for the web server.")
	newRemoteCmd.Flags().String("git", "", "Set the user, address and port for the git server.")
	return newRemoteCmd
}
