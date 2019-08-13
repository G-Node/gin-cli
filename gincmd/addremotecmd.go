package gincmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ginclient "github.com/G-Node/gin-cli/ginclient"
	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/G-Node/gin-cli/git"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Remote type
type rtype int8

const (
	// ginrt: GIN repository
	ginrt rtype = iota
	// dirrt: Directory remote (path to a local directory)
	dirrt
	// unknownrt: Any other kind of git server
	unknownrt
)

type remote struct {
	rt rtype

	// server is one of: "gin", "dir", or a git server URL (for unknownrt)
	server string

	// path is the repository path provided by the user
	// if the server is "gin" or a git server it's of the form <username>/<repositoryname>
	// for "dir" type remotes, this is the directory path as supplied by the user
	path string

	// url is the full repository URL including username and protocol (e.g., ssh://git@gin.g-node.org:22/<username>/<repositoryname>)
	// for unknown remote types, this is equivalent to path
	// for "dir" type remotes, this is the absolute path of the directory supplied by the user
	url string
}

const allremotes = "all"

func splitAliasRemote(remote string) (string, string) {
	// split on first colon and check if it's a known alias
	parts := strings.SplitN(remote, ":", 2)
	if len(parts) < 2 {
		Die("remote location must be of the form <server>:<repositoryname>, or <alias>:<repositoryname> (see \"gin help add-remote\")")
	}
	return parts[0], parts[1]
}

func parseRemote(remotestr string) remote {
	var rmt remote
	rmt.server, rmt.path = splitAliasRemote(remotestr)
	if rmt.server == "dir" {
		rmt.rt = dirrt
		rmt.url, _ = filepath.Abs(rmt.path)
		return rmt
	}

	conf := config.Read()
	if srvcfg, ok := conf.Servers[rmt.server]; ok {
		rmt.url = fmt.Sprintf("%s/%s", srvcfg.Git.AddressStr(), rmt.path)
		rmt.rt = ginrt
		return rmt
	}

	// Unknown alias, return as is
	rmt.rt = unknownrt
	rmt.server = rmt.path
	return rmt
}

func checkRemote(cmd *cobra.Command, url string) (err error) {
	// Check if the remote is accessible
	fmt.Print(":: Checking remote: ")
	if _, err = git.LsRemote(url); err == nil {
		fmt.Fprintln(color.Output, green("OK"))
		return nil
	}
	fmt.Println("not found")
	return err
}

func createGinRemote(cmd *cobra.Command, rmt remote) {
	gincl := ginclient.New(rmt.server)
	requirelogin(cmd, gincl, true)
	repopathParts := strings.SplitN(rmt.path, "/", 2)
	reponame := repopathParts[1]
	fmt.Printf(":: Creating repository '%s' ", rmt.path)
	err := gincl.CreateRepo(reponame, "")
	CheckError(err)
	fmt.Fprintln(color.Output, green("OK"))
}

func createDirRemote(rmt remote) {
	origdir, err := os.Getwd()
	CheckError(err)
	defer os.Chdir(origdir)
	err = os.MkdirAll(rmt.url, 0755)
	if err != nil {
		Die(fmt.Sprintf("Directory remote creation failed: %v", err))
	}
	os.Chdir(rmt.url)
	gincl := ginclient.New("")
	err = gincl.InitDir(true)
	CheckError(err)
	git.AnnexDescribe("here", "GIN Storage")
}

func createRemote(cmd *cobra.Command, rmt remote) {
	switch rmt.rt {
	case ginrt:
		createGinRemote(cmd, rmt)
	case dirrt:
		createDirRemote(rmt)
	default:
		// unknown remotes are not yet supported
		Die(fmt.Sprintf("type or server '%s' unknown: cannot create remote", rmt.server))
	}
}

func promptCreate(cmd *cobra.Command, rmt remote) {
	var response string
	fmt.Printf("Remote %s does not exist. Would you like to create it?\n", rmt.url)
	for {
		fmt.Printf("[c]reate / [a]dd anyway / a[b]ort: ")
		fmt.Scanln(&response)

		switch strings.ToLower(response) {
		case "c", "create":
			createRemote(cmd, rmt)
			return
		case "a", "add", "add anyway":
			return
		case "b", "abort":
			Exit("aborted")
		}
	}
}

func defaultRemoteIfUnset(name string) {
	_, err := ginclient.DefaultRemote()
	if err != nil {
		ginclient.SetDefaultRemote(name)
	}
}

func addRemote(cmd *cobra.Command, args []string) {
	if !git.Checkwd() {
		Die(ginerrors.NotInRepo)
	}
	flags := cmd.Flags()
	nocreateprompt, _ := flags.GetBool("create")
	setdefault, _ := flags.GetBool("default")
	name, remotestr := args[0], args[1]
	if name == allremotes {
		Die("cannot set a remote with name 'all': see 'gin help add-remote' and 'gin help upload'")
	}

	// TODO: Check if remote with same name already exists; fail early
	rmt := parseRemote(remotestr)
	err := checkRemote(cmd, rmt.url)
	// TODO: Check if it's a gin URL before offering to create
	if err != nil {
		if nocreateprompt {
			createRemote(cmd, rmt)
		} else {
			promptCreate(cmd, rmt)
		}
	}
	err = git.RemoteAdd(name, rmt.url)
	CheckError(err)
	fmt.Printf(":: Added new remote: %s [%s]\n", name, rmt.url)
	if setdefault {
		ginclient.SetDefaultRemote(name)
	} else {
		defaultRemoteIfUnset(name)
	}
	defremote, err := ginclient.DefaultRemote()
	CheckError(err)
	fmt.Printf(":: Default remote: %s\n", defremote)
}

// AddRemoteCmd sets up the 'add-remote' repository subcommand
func AddRemoteCmd() *cobra.Command {
	description := `Add a remote to the current repository for uploading and downloading. The name of the remote can be any word except the reserved keyword 'all' (reserved for performing uploads to all configured remotes).

The location must be of the form alias:path or server:path. Currently supported aliases are 'gin' for the default configured gin server, and 'dir' for directories. If neither is specified, it is assumed to be the address of a git server. For gin remotes, the path is the location of the repository on the server, in the form user/repositoryname. For directories, it is the path to the storage directory.

When a remote is added, if it does not exist, the client will offer to create it. This is only possible for 'gin' and 'dir' type remotes and any other GIN servers the user has configured.

A new remote is set as the default for uploading if no other remotes are configured. To set any new remote as the default, use the --default option. Use the 'use-remote' command to change the default remote at any time.`

	// When a remote is added, if it does not exist, the client will offer to create it. This is only possible for 'gin' and 'dir' remotes and any other GIN servers the user has configured.`
	args := map[string]string{
		"<name>":     "The name of the new remote",
		"<location>": "The location of the data store, in the form alias:path or server:path",
	}
	examples := map[string]string{
		"Add a GIN server repository as a remote named 'primary'":          "$ gin add-remote primary gin:alice/example",
		"Add a directory on a storage drive as a remote named 'datastore'": "$ gin add-remote datastore dir:/mnt/gindatastore",
	}
	var cmd = &cobra.Command{
		Use:                   "add-remote <name> <location>",
		Short:                 "Add a remote to the current repository for uploading and downloading",
		Long:                  formatdesc(description, args),
		Example:               formatexamples(examples),
		Args:                  cobra.ExactArgs(2),
		Run:                   addRemote,
		DisableFlagsInUseLine: true,
	}
	cmd.Flags().Bool("create", false, "Create the remote on the server if it does not already exist.")
	cmd.Flags().Bool("default", false, "Sets the new remote as the default (if the command succeeds).")
	return cmd
}
