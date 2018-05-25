package gincmd

import (
	"fmt"
	"os"
	"strings"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type rtype int8

const (
	ginrt rtype = iota
	dirrt
	unknownrt
)

func splitAliasRemote(remote string) (string, string) {
	// split on first colon and check if it's a known alias
	parts := strings.SplitN(remote, ":", 2)
	if len(parts) < 2 {
		Die("remote location must be of the form <server>:<repositoryname>, or <alias>:<repositoryname> (see \"gin help add-remote\")")
	}
	return parts[0], parts[1]
}

func parseRemote(remote string) (rtype, string) {
	alias, repopath := splitAliasRemote(remote)
	switch alias {
	// TODO: Support configurable aliases for alternative GIN servers (self hosted), which should return 'ginrt'
	case "gin":
		// Built-in alias 'gin'; use default remote address
		conf := config.Read()
		url := fmt.Sprintf("ssh://%s@%s/%s", conf.GitUser, conf.GitHost, repopath)
		return ginrt, url
	case "dir":
		// Built-in alias 'dir'; set up filesystem directory as bare remote
		return dirrt, repopath
	}
	// Unknown alias, return as is
	return unknownrt, remote
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

func createGinRemote(cmd *cobra.Command, repopath string) {
	conf := config.Read()
	gincl := ginclient.New(conf.GinHost)
	requirelogin(cmd, gincl, true)
	repopathParts := strings.SplitN(repopath, "/", 2)
	reponame := repopathParts[1]
	fmt.Printf(":: Creating repository '%s' ", repopath)
	err := gincl.CreateRepo(reponame, "")
	CheckError(err)
	fmt.Fprintln(color.Output, green("OK"))
}

func createDirRemote(repopath string) {
	origdir, err := os.Getwd()
	CheckError(err)
	defer os.Chdir(origdir)
	os.MkdirAll(repopath, 0755)
	os.Chdir(repopath)
	gincl := ginclient.New("")
	err = gincl.InitDir(true)
	CheckError(err)
	git.AnnexDescribe("here", "GIN Storage")
}

func createRemote(cmd *cobra.Command, rt rtype, remote string) {
	alias, repopath := splitAliasRemote(remote)
	switch rt {
	case ginrt:
		createGinRemote(cmd, repopath)
	case dirrt:
		createDirRemote(repopath)
	default:
		Die(fmt.Sprintf("type or server '%s' unknown: cannot create remote", alias))
	}
}

func promptCreate(cmd *cobra.Command, rt rtype, remote string) {
	var response string
	fmt.Printf("Remote %s does not exist. Would you like to create it?\n", remote)
	for {
		fmt.Printf("[c]reate / [a]dd anyway / a[b]ort: ")
		fmt.Scanln(&response)

		switch strings.ToLower(response) {
		case "c", "create":
			createRemote(cmd, rt, remote)
			return
		case "a", "add", "add anyway":
			return
		case "b", "abort":
			Exit("aborted")
		}
	}
}

func defaultIfUnset(name string) {
	_, err := ginclient.DefaultRemote()
	if err != nil {
		ginclient.SetDefaultRemote(name)
	}
}

func addRemote(cmd *cobra.Command, args []string) {
	if !git.IsRepo() {
		Die(ginerrors.NotInRepo)
	}
	flags := cmd.Flags()
	create, _ := flags.GetBool("create")
	name, remote := args[0], args[1]
	rt, url := parseRemote(remote)
	err := checkRemote(cmd, url)
	// TODO: Check if it's a gin URL before offering to create
	if err != nil {
		if create {
			createRemote(cmd, rt, remote)
		} else {
			promptCreate(cmd, rt, remote)
		}
	}
	err = git.RemoteAdd(name, url)
	CheckError(err)
	fmt.Printf(":: Added new remote: %s [%s]\n", name, url)
	defaultIfUnset(name)
	defremote, err := ginclient.DefaultRemote()
	CheckError(err)
	fmt.Printf(":: Default remote: %s\n", defremote)
}

// AddRemoteCmd sets up the 'add-remote' repository subcommand
func AddRemoteCmd() *cobra.Command {
	description := `Add a remote to the current repository for uploading and downloading. The name of the remote can be any word except the reserved keyword 'all'.

The location must be of the form alias:path or server:path. Currently supported aliases are 'gin' for the default configured gin server, and 'dir' for directories. If neither is specified, it is assumed to be the address of a git server. For gin remotes, the path is the location of the repository on the server, in the form user/repositoryname. For directories, it is the path to the storage directory.

When a remote is added, if it does not exist on the server or in the specified directory, the client will offer to create it. This is only possible for 'gin' and 'dir' remotes.

A new remote is set as the default for uploading if no other remotes are configured. To set any new remote as the default, use the --default option. Use the 'set-remote' command to change the default remote at any time.`

	// When a remote is added, if it does not exist, the client will offer to create it. This is only possible for 'gin' and 'dir' remotes and any other GIN servers the user has configured.`
	args := map[string]string{
		"<name>":     "The name of the new remote",
		"<location>": "The location of the data store, in the form alias:path or server:path",
	}
	examples := map[string]string{
		"Add a GIN server repository as a remote named 'primary'":          "$ gin add-remote primary gin:alice/example",
		"Add a directory on a storage drive as a remote named 'datastore'": "$ gin add-remote datastore dir:/mnt/gindatastore",
	}
	var addRemoteCmd = &cobra.Command{
		Use:     "add-remote <name> <location>",
		Short:   "Add a remote to the current repository for uploading and downloading.",
		Long:    formatdesc(description, args),
		Example: formatexamples(examples),
		Args:    cobra.ExactArgs(2),
		Run:     addRemote,
		DisableFlagsInUseLine: true,
	}
	addRemoteCmd.Flags().Bool("create", false, "Create the remote on the server if it does not already exist.")
	addRemoteCmd.Flags().Bool("default", false, "Sets the new remote as the default (if the command succeeds).")
	return addRemoteCmd
}
