package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/bbrks/wrap"
	"github.com/fatih/color"
	gogs "github.com/gogits/go-gogs-client"
	version "github.com/hashicorp/go-version"
	"github.com/howeyc/gopass"
	"github.com/spf13/cobra"
)

var gincliversion string
var build string
var commit string
var verstr string
var minAnnexVersion = "6.20160126" // Introduction of git-annex add --json

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

// login requests credentials, performs login with auth server, and stores the token.
func login(cmd *cobra.Command, args []string) {
	var username string
	var password string

	if args != nil && len(args) == 0 {
		// prompt for login
		fmt.Print("Login: ")
		fmt.Scanln(&username)
	} else if len(args) > 1 {
		usageDie(cmd)
	} else {
		username = args[0]
	}

	// prompt for password
	fmt.Print("Password: ")
	pwbytes, err := gopass.GetPasswdMasked()
	fmt.Println()
	if err != nil {
		// read error or gopass.ErrInterrupted
		if err == gopass.ErrInterrupted {
			util.Die("Cancelled.")
		}
		if err == gopass.ErrMaxLengthExceeded {
			util.Die("Input too long")
		}
		util.Die(err)
	}

	password = string(pwbytes)

	if password == "" {
		util.Die("No password provided. Aborting.")
	}

	gincl := ginclient.NewClient(util.Config.GinHost)
	err = gincl.Login(username, password, "gin-cli")
	util.CheckError(err)
	info, err := gincl.RequestAccount(username)
	util.CheckError(err)
	fmt.Printf("Hello %s. You are now logged in.\n", info.UserName)
}

func logout(cmd *cobra.Command, args []string) {
	if len(args) > 0 {
		usageDie(cmd)
	}
	gincl := ginclient.NewClient(util.Config.GinHost)
	err := gincl.LoadToken()
	if err != nil {
		util.Die("You are not logged in.")
	}

	gincl.Logout()
	fmt.Println("You have been logged out.")
}

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
		printProgress(initchan, false)
	} else if !noclone {
		// Clone repository after creation
		getRepo(cmd, []string{repoPath})
	}
}

func deleteRepo(cmd *cobra.Command, args []string) {
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, true)
	var repostr, confirmation string

	if len(args) == 0 {
		usageDie(cmd)
	} else {
		repostr = args[0]
	}

	repoinfo, err := gincl.GetRepo(repostr)
	util.CheckError(err)

	if repoinfo.FullName != repostr {
		util.LogWrite("ERROR: Mismatch in repository names: %s != %s", repoinfo.FullName, repostr)
		util.Die("An unexpected error occurred while communicating with the server.")
	}

	fmt.Println("--- WARNING ---")
	fmt.Println("You are about to delete a remote repository, all its files, and history.")
	fmt.Println("This action is irreversible.")

	fmt.Println("If you are sure you want to delete this repository, type its full name (owner/name) below")
	fmt.Print("> ")
	fmt.Scanln(&confirmation)

	if repoinfo.FullName == confirmation && repostr == confirmation {
		err = gincl.DelRepo(repostr)
		util.CheckError(err)
	} else {
		util.Die("Confirmation does not match repository name. Cancelling.")
	}

	fmt.Printf("Repository %s has been deleted!\n", repostr)
}

func isValidRepoPath(path string) bool {
	return strings.Contains(path, "/")
}

func getRepo(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	repostr := args[0]
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, !jsonout)

	if !isValidRepoPath(repostr) {
		util.Die(fmt.Sprintf("Invalid repository path '%s'. Full repository name should be the owner's username followed by the repository name, separated by a '/'.\nType 'gin help get' for information and examples.", repostr))
	}

	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	clonechan := make(chan ginclient.RepoFileStatus)
	go gincl.CloneRepo(repostr, clonechan)
	printProgress(clonechan, jsonout)
}

func lsRepo(cmd *cobra.Command, args []string) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	flags := cmd.Flags()
	if flags.NFlag() > 1 {
		usageDie(cmd)
	}
	jsonout, _ := flags.GetBool("json")
	short, _ := flags.GetBool("short")

	gincl := ginclient.NewClient(util.Config.GinHost)
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser

	filesStatus, err := gincl.ListFiles(args...)
	util.CheckError(err)

	if short {
		for fname, status := range filesStatus {
			fmt.Printf("%s %s\n", status.Abbrev(), fname)
		}
	} else if jsonout {
		type fstat struct {
			FileName string `json:"filename"`
			Status   string `json:"status"`
		}
		var statuses []fstat
		for fname, status := range filesStatus {
			statuses = append(statuses, fstat{FileName: fname, Status: status.Abbrev()})
		}
		jsonbytes, err := json.Marshal(statuses)
		util.CheckError(err)
		fmt.Println(string(jsonbytes))
	} else {
		// Files are printed separated by status and sorted by name
		statFiles := make(map[ginclient.FileStatus][]string)

		for file, status := range filesStatus {
			statFiles[status] = append(statFiles[status], file)
		}

		// sort files in each status (stable sorting unnecessary)
		// also collect active statuses for sorting
		var statuses ginclient.FileStatusSlice
		for status := range statFiles {
			sort.Sort(sort.StringSlice(statFiles[status]))
			statuses = append(statuses, status)
		}
		sort.Sort(statuses)

		// print each category with len(items) > 0 with appropriate header
		for _, status := range statuses {
			fmt.Printf("%s:\n", status.Description())
			fmt.Printf("\n\t%s\n\n", strings.Join(statFiles[status], "\n\t"))
		}
	}
}

func lock(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	gincl := ginclient.NewClient(util.Config.GinHost)
	lockchan := make(chan ginclient.RepoFileStatus)
	go gincl.LockContent(args, lockchan)
	printProgress(lockchan, jsonout)
}

func unlock(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	gincl := ginclient.NewClient(util.Config.GinHost)
	unlockchan := make(chan ginclient.RepoFileStatus)
	go gincl.UnlockContent(args, unlockchan)
	printProgress(unlockchan, jsonout)
}

func printProgress(statuschan <-chan ginclient.RepoFileStatus, jsonout bool) {
	var fname, state string
	var lastprint string
	filesuccess := make(map[string]bool)
	for stat := range statuschan {
		var msgparts []string
		if jsonout {
			j, _ := json.Marshal(stat)
			fmt.Println(string(j))
			continue
		}
		if stat.FileName != fname || stat.State != state {
			// New line if new file or new state
			if len(lastprint) > 0 {
				fmt.Println()
			}
			lastprint = ""
			fname = stat.FileName
			state = stat.State
		}
		msgparts = append(msgparts, stat.State, stat.FileName)
		if stat.Err == nil {
			if stat.Progress == "100%" {
				msgparts = append(msgparts, green("OK"))
				filesuccess[stat.FileName] = true
			} else {
				msgparts = append(msgparts, stat.Progress, stat.Rate)
			}
		} else {
			msgparts = append(msgparts, red(stat.Err.Error()))
			filesuccess[stat.FileName] = false
		}
		newprint := fmt.Sprintf("\r%s", util.CleanSpaces(strings.Join(msgparts, " ")))
		if newprint != lastprint {
			fmt.Printf("\r%s", strings.Repeat(" ", len(lastprint))) // clear the line
			fmt.Fprint(color.Output, newprint)
			lastprint = newprint
		}
	}
	if len(lastprint) > 0 {
		fmt.Println()
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

func upload(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, !jsonout)
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser

	lockchan := make(chan ginclient.RepoFileStatus)
	go gincl.LockContent(args, lockchan)
	printProgress(lockchan, jsonout)
	uploadchan := make(chan ginclient.RepoFileStatus)
	go gincl.Upload(args, uploadchan)
	printProgress(uploadchan, jsonout)
}

func download(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, !jsonout)
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	content, _ := cmd.Flags().GetBool("content")
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	lockchan := make(chan ginclient.RepoFileStatus)
	go gincl.LockContent([]string{}, lockchan)
	printProgress(lockchan, jsonout)
	if !jsonout {
		fmt.Print("Downloading changes ")
	}
	err := gincl.Download()
	util.CheckError(err)
	if !jsonout {
		fmt.Fprintln(color.Output, green("OK"))
	}
	if content {
		reporoot, _ := util.FindRepoRoot(".")
		ginclient.Workingdir = reporoot
		getContent(cmd, nil)
	}
}

func getContent(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, !jsonout)
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	getcchan := make(chan ginclient.RepoFileStatus)
	go gincl.GetContent(args, getcchan)
	printProgress(getcchan, jsonout)
}

func remove(cmd *cobra.Command, args []string) {
	jsonout, _ := cmd.Flags().GetBool("json")
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, !jsonout)
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	lockchan := make(chan ginclient.RepoFileStatus)
	go gincl.LockContent(args, lockchan)
	printProgress(lockchan, jsonout)
	rmchan := make(chan ginclient.RepoFileStatus)
	go gincl.RemoveContent(args, rmchan)
	printProgress(rmchan, jsonout)
}

func keys(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	if flags.NFlag() > 1 {
		usageDie(cmd)
	}

	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, true)

	keyfilename, _ := flags.GetString("add")
	keyidx, _ := flags.GetInt("delete")
	verbose, _ := flags.GetBool("verbose")

	if keyfilename != "" {
		addKey(gincl, keyfilename)
		return
	}
	if keyidx > 0 {
		delKey(gincl, keyidx)
		return
	}
	printKeys(gincl, verbose)
}

func printKeys(gincl *ginclient.Client, verbose bool) {
	keys, err := gincl.GetUserKeys()
	util.CheckError(err)

	nkeys := len(keys)
	var plural string
	if nkeys == 1 {
		plural = ""
	} else {
		plural = "s"
	}

	var nkeysStr string
	if nkeys == 0 {
		nkeysStr = "no"
	} else {
		nkeysStr = fmt.Sprintf("%d", nkeys)
	}
	fmt.Printf("You have %s key%s associated with your account.\n\n", nkeysStr, plural)
	for idx, key := range keys {
		fmt.Printf("[%v] \"%s\"\n", idx+1, key.Title)
		if verbose {
			fmt.Printf("--- Key ---\n%s\n", key.Key)
		}
	}
}

func addKey(gincl *ginclient.Client, filename string) {
	keyBytes, err := ioutil.ReadFile(filename)
	util.CheckError(err)
	key := string(keyBytes)
	strSlice := strings.Split(key, " ")
	var description string
	if len(strSlice) > 2 {
		description = strings.TrimSpace(strSlice[2])
	} else {
		description = fmt.Sprintf("%s@%s", gincl.Username, strconv.FormatInt(time.Now().Unix(), 10))
	}

	err = gincl.AddKey(string(keyBytes), description, false)
	util.CheckError(err)
	fmt.Printf("New key added '%s'\n", description)
}

func delKey(gincl *ginclient.Client, idx int) {
	name, err := gincl.DeletePubKeyByIdx(idx)
	util.CheckError(err)
	fmt.Printf("Deleted key with name '%s'\n", name)
}

func printAccountInfo(cmd *cobra.Command, args []string) {
	var username string

	gincl := ginclient.NewClient(util.Config.GinHost)
	_ = gincl.LoadToken() // does not REQUIRE login

	if len(args) == 0 {
		username = gincl.Username
	} else {
		username = args[0]
	}

	if username == "" {
		// prompt for username
		fmt.Print("Specify username for info lookup: ")
		username = ""
		fmt.Scanln(&username)
	}

	info, err := gincl.RequestAccount(username)
	util.CheckError(err)

	var outBuffer bytes.Buffer
	_, _ = outBuffer.WriteString(fmt.Sprintf("User %s\nName: %s\n", info.UserName, info.FullName))
	if info.Email != "" {
		_, _ = outBuffer.WriteString(fmt.Sprintf("Email: %s\n", info.Email))
	}

	fmt.Println(outBuffer.String())
}

func printRepoList(repolist []gogs.Repository) {
	for _, repo := range repolist {
		fmt.Printf("* %s\n", repo.FullName)
		fmt.Printf("\tLocation: %s\n", repo.HTMLURL)
		desc := strings.Trim(repo.Description, "\n")
		if desc != "" {
			fmt.Printf("\tDescription: %s\n", desc)
		}
		if repo.Website != "" {
			fmt.Printf("\tWebsite: %s\n", repo.Website)
		}
		if !repo.Private {
			fmt.Println("\tThis repository is public")
		}
		fmt.Println()
	}
}

func repos(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	jsonout, _ := flags.GetBool("json")
	allrepos, _ := flags.GetBool("all")
	sharedrepos, _ := flags.GetBool("shared")
	if (allrepos && sharedrepos) || ((allrepos || sharedrepos) && len(args) > 0) {
		usageDie(cmd)
	}
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(cmd, gincl, true)
	username := gincl.Username
	if len(args) == 1 && args[0] != username {
		username = args[0]
		// for other users, print everything
		allrepos = true
	}
	repolist, err := gincl.ListRepos(username)
	util.CheckError(err)

	var userrepos []gogs.Repository
	var otherrepos []gogs.Repository

	for _, repo := range repolist {
		if repo.Owner.UserName == gincl.Username {
			userrepos = append(userrepos, repo)
		} else {
			otherrepos = append(otherrepos, repo)
		}
	}

	if jsonout {
		var outlist []gogs.Repository
		if allrepos {
			outlist = append(userrepos, otherrepos...)
		} else if sharedrepos {
			outlist = otherrepos
		} else {
			outlist = userrepos
		}
		if len(outlist) > 0 {
			j, _ := json.Marshal(outlist)
			fmt.Println(string(j))
		}
		return
	}

	printedlines := 0
	if len(userrepos) > 0 && !sharedrepos {
		printedlines += len(userrepos)
		printRepoList(userrepos)
	}
	if len(otherrepos) > 0 && (sharedrepos || allrepos) {
		printedlines += len(otherrepos)
		printRepoList(otherrepos)
	}

	if printedlines == 0 {
		fmt.Println("No repositories found")
	}
}

// help prints the help text for either a specific command, if args has one string that is a known command, or the general usage text otherwise.
func help(args []string) {
	var helptext string
	var ok bool
	if len(args) != 1 {
		helptext = usage
	} else {
		helptext, ok = cmdHelp[args[0]]
		if !ok {
			helptext = usage
		}
	}
	fmt.Println(helptext)
}

func usageDie(cmd *cobra.Command) {
	cmd.Help()
	// exit without message
	util.Die("")
}

func checkAnnexVersion() {
	errmsg := fmt.Sprintf("The GIN Client requires git-annex %s or newer", minAnnexVersion)
	verstring, err := ginclient.GetAnnexVersion()
	util.CheckError(err)
	systemver, err := version.NewVersion(verstring)
	if err != nil {
		// Special case for neurodebian git-annex version
		// The versionn string contains a tilde as a separator for the arch suffix
		// Cutting off the suffix and checking again
		verstring = strings.Split(verstring, "~")[0]
		systemver, err = version.NewVersion(verstring)
		if err != nil {
			// Can't figure out the version. Giving up.
			util.Die(fmt.Sprintf("%s\ngit-annex version %s not understood", errmsg, verstring))
		}
	}
	minver, _ := version.NewVersion(minAnnexVersion)
	if systemver.LessThan(minver) {
		util.Die(fmt.Sprintf("%s\nFound version %s", errmsg, verstring))
	}
}

func gitrun(cmd *cobra.Command, args []string) {
	gincl := ginclient.NewClient(util.Config.GinHost)
	_ = gincl.LoadToken() // OK to run without token

	gitcmd, err := ginclient.RunGitCommand(args...)
	util.CheckError(err)
	for {
		line, readerr := gitcmd.OutPipe.ReadLine()
		if readerr != nil {
			break
		}
		fmt.Print(line)
	}
	fmt.Print(gitcmd.ErrPipe.ReadAll())
	err = gitcmd.Wait()
	if err != nil {
		os.Exit(1)
	}
}

func annexrun(cmd *cobra.Command, args []string) {
	gincl := ginclient.NewClient(util.Config.GinHost)
	_ = gincl.LoadToken() // OK to run without token
	annexcmd, err := ginclient.RunAnnexCommand(args...)
	util.CheckError(err)
	var line string
	for {
		line, err = annexcmd.OutPipe.ReadLine()
		if err != nil {
			break
		}
		fmt.Print(line)
	}
	fmt.Print(annexcmd.ErrPipe.ReadAll())
	err = annexcmd.Wait()
	if err != nil {
		os.Exit(1)
	}
}

func printversion(args []string) {
	jsonout := true // TODO: make this a new command
	if jsonout {
		verjson := struct {
			Version string `json:"version"`
			Build   string `json:"build"`
			Commit  string `json:"commit"`
		}{
			gincliversion,
			build,
			commit,
		}
		verjsonstr, _ := json.Marshal(verjson)
		fmt.Println(string(verjsonstr))
	} else {
		fmt.Println(verstr)
	}
}

func init() {
	if gincliversion == "" {
		verstr = "GIN command line client [dev build]"
	} else {
		verstr = fmt.Sprintf("GIN command line client %s Build %s (%s)", gincliversion, build, commit)
	}
}

func main() {
	err := util.LogInit(verstr)
	util.CheckError(err)
	defer util.LogClose()

	err = util.LoadConfig()
	util.CheckError(err)
	checkAnnexVersion()

	var rootCmd = &cobra.Command{
		Use:                   "gin",
		Long:                  "GIN Command Line Interface and client for the GIN services", // TODO: Add license and web info
		Version:               fmt.Sprintln(verstr),
		DisableFlagsInUseLine: true,
	}
	rootCmd.SetVersionTemplate("{{ .Version }}")

	// TODO: Nicely wrap long command descriptions
	// TODO: Add argument descriptions (might have to put them in the long description)
	// cobra.AddTemplateFunc("indent", indent)
	// cobra.AddTemplateFunc("wrapflags", wrapFlags)
	// cobra.AddTemplateFunc("wrapdesc", wrapDescription)
	// cobra.AddTemplateFunc("wrapexample", wrapExample)

	// rootCmd.SetHelpTemplate(usageTemplate)

	// Login
	var loginCmd = &cobra.Command{
		Use:   "login [<username>]",
		Short: "Login to the GIN services",
		Long:  "Login to the GIN services.",
		Args:  cobra.MaximumNArgs(1),
		Run:   login,
		DisableFlagsInUseLine: true,
	}
	rootCmd.AddCommand(loginCmd)

	// Logout
	var logoutCmd = &cobra.Command{
		Use:   "logout",
		Short: "Logout of the GIN services",
		Long:  "Logout of the GIN services. This command takes no arguments.",
		Args:  cobra.NoArgs,
		Run:   logout,
		DisableFlagsInUseLine: true,
	}
	rootCmd.AddCommand(logoutCmd)

	// Create repo
	var createCmd = &cobra.Command{
		Use:   "create [--here | --no-clone] [<repository>] [<description>]",
		Short: "Create a new repository on the GIN server",
		Long:  "Create a new repository on the GIN server and optionally clone it locally or initialise working directory.",
		Args:  cobra.MaximumNArgs(2),
		Run:   createRepo,
		DisableFlagsInUseLine: true,
	}
	createCmd.Flags().Bool("here", false, "Create the local repository clone in the current working directory. Cannot be used with --no-clone.")
	createCmd.Flags().Bool("no-clone", false, "Create repository on the server but do not clone it locally. Cannot be used with --here.")
	rootCmd.AddCommand(createCmd)

	// Delete repo (unlisted)
	var deleteCmd = &cobra.Command{
		Use:   "delete <repository>",
		Short: "Delete a repository from the GIN server",
		Long:  "Delete a repository from the GIN server.",
		Args:  cobra.ExactArgs(1),
		Run:   deleteRepo,
		DisableFlagsInUseLine: true,
		Hidden:                true,
	}
	rootCmd.AddCommand(deleteCmd)

	// Get repo
	var getRepoCmd = &cobra.Command{
		Use:   "get [--json] <repository>",
		Short: "Retrieve (clone) a repository from the remote server",
		Long:  "Download a remote repository to a new directory and initialise the directory with the default options. The local directory is referred to as the 'clone' of the repository.",
		Args:  cobra.ExactArgs(1),
		Run:   getRepo,
		DisableFlagsInUseLine: true,
	}
	createCmd.Flags().Bool("json", false, "Print output in JSON format.")
	rootCmd.AddCommand(getRepoCmd)

	// List files
	var lsRepoCmd = &cobra.Command{
		Use:   "ls [--json | --short | -s] [<filenames>]...",
		Short: "List the sync status of files in the local repository",
		Long:  "List one or more files or the contents of directories and their status. With no arguments, lists the status of the files under the current directory. Directory listings are performed recursively.",
		Args:  cobra.ArbitraryArgs,
		Run:   lsRepo,
		DisableFlagsInUseLine: true,
	}
	lsRepoCmd.Flags().Bool("json", false, "Print listing in JSON format (uses short form abbreviations).")
	lsRepoCmd.Flags().BoolP("short", "s", false, "Print listing in short form.")
	rootCmd.AddCommand(lsRepoCmd)

	w := wrap.NewWrapper()

	// Unlock content
	var unlockCmd = &cobra.Command{
		Use:   "unlock [--json] [<filenames>]...",
		Short: "Unlock files for editing",
		Long:  w.Wrap("Unlock one or more files for editing. Files added to the repository are left in a locked state, which allows reading but prevents editing. In order to edit or write to a file, it must first be unlocked. When done editing, it is recommended that the file be locked again using the 'lock' command.\n\nAfter performing an 'upload, 'download', or 'get', affected files are always reverted to the locked state.\n\nUnlocking a file takes longer depending on its size.", 80),
		Args:  cobra.ArbitraryArgs,
		Run:   unlock,
		DisableFlagsInUseLine: true,
	}
	unlockCmd.Flags().Bool("json", false, "Print output in JSON format.")
	rootCmd.AddCommand(unlockCmd)

	// Lock content
	var lockCmd = &cobra.Command{
		Use:   "lock [--json] [<filenames>]...",
		Short: "Lock files",
		Long:  w.Wrap("Lock one or more files after editing. After unlocking files for editing (using the 'unlock' command), it is recommended that they be locked again. This records any changes made and prepares a file for upload.\n\nLocked files are replaced by symbolic links in the working directory (where supported by the filesystem).\n\nAfter performing an 'upload', 'download', or 'get', affected files are reverted to a locked state.\n\nLocking a file takes longer depending on the size of the file.", 80),
		Args:  cobra.ArbitraryArgs,
		Run:   lock,
		DisableFlagsInUseLine: true,
	}
	lockCmd.Flags().Bool("json", false, "Print output in JSON format.")
	rootCmd.AddCommand(lockCmd)

	// Upload
	var uploadCmd = &cobra.Command{
		Use:   "upload [--json] [<filenames>]...",
		Short: "Upload local changes to a remote repository",
		Long:  w.Wrap("Upload changes made in a local repository clone to the remote repository on the GIN server. This command must be called from within the local repository clone. Specific files or directories may be specified. All changes made will be sent to the server, including addition of new files, modifications and renaming of existing files, and file deletions.\n\nIf no arguments are specified, only changes to files already being tracked are uploaded.", 80),
		Args:  cobra.ArbitraryArgs,
		Run:   upload,
		DisableFlagsInUseLine: true,
	}
	uploadCmd.Flags().Bool("json", false, "Print output in JSON format.")
	rootCmd.AddCommand(uploadCmd)

	// Download
	var downloadCmd = &cobra.Command{
		Use:   "download [--json] [--content]",
		Short: "Download all new information from a remote repository",
		Long:  w.Wrap("Downloads changes from the remote repository to the local clone. This will create new files that were added remotely, delete files that were removed, and update files that were changed.\n\nOptionally downloads the content of all files in the repository. If 'content' is not specified, new files will be empty placeholders. Content of individual files can later be retrieved using the 'get-content' command.", 80),
		Args:  cobra.NoArgs,
		Run:   download,
		DisableFlagsInUseLine: true,
	}
	downloadCmd.Flags().Bool("json", false, "Print output in JSON format.")
	downloadCmd.Flags().Bool("content", false, "Download the content for all files in the repository.")
	rootCmd.AddCommand(downloadCmd)

	// Get content
	var getContentCmd = &cobra.Command{
		Use:                   "get-content [--json] [<filenames>]...",
		Short:                 "Download the content of files from a remote repository",
		Long:                  w.Wrap("Download the content of the listed files. The get-content command is intended to be used to retrieve the content of placeholder files in a local repository. This command must be called from within the local repository clone. With no arguments, downloads the content for all files under the working directory, recursively.", 80),
		Args:                  cobra.ArbitraryArgs,
		Run:                   getContent,
		Aliases:               []string{"getc"},
		DisableFlagsInUseLine: true,
	}
	getContentCmd.Flags().Bool("json", false, "Print output in JSON format.")
	rootCmd.AddCommand(getContentCmd)

	// Remove content
	var rmContentCmd = &cobra.Command{
		Use:                   "remove-content [--json] [<filenames>]...",
		Short:                 "Remove the content of local files that have already been uploaded",
		Long:                  w.Wrap("Remove the content of local files. This command will not remove the content of files that have not been already uploaded to a remote repository, even if the user specifies such files explicitly. Removed content can be retrieved from the server by using the 'get-content' command. With no arguments, removes the content of all files under the current working directory, as long as they have been safely uploaded to a remote repository.\n\nNote that after removal, placeholder files will remain in the local repository. These files appear as 'No Content' when running the 'gin ls' command.", 80),
		Args:                  cobra.ArbitraryArgs,
		Run:                   remove,
		Aliases:               []string{"rmc"},
		DisableFlagsInUseLine: true,
	}
	rmContentCmd.Flags().Bool("json", false, "Print output in JSON format.")
	rootCmd.AddCommand(rmContentCmd)

	// Account info
	var infoCmd = &cobra.Command{
		Use:   "info [username]",
		Short: "Print a user's information",
		Long:  "Print user information. If no argument is provided, it will print the information of the currently logged in user. Using this command with no argument can also be used to check if a user is currently logged in.",
		Args:  cobra.MaximumNArgs(1),
		Run:   printAccountInfo,
		DisableFlagsInUseLine: true,
	}
	rootCmd.AddCommand(infoCmd)

	// List repos
	var reposCmd = &cobra.Command{
		Use:   "repos [--shared | --all | <username>]",
		Short: "List available remote repositories",
		Long:  w.Wrap("List repositories on the server that provide read access. If no argument is provided, it will list the repositories owned by the logged in user.\n\nNote that only one of the options can be specified.", 80),
		Args:  cobra.MaximumNArgs(1),
		Run:   repos,
		DisableFlagsInUseLine: true,
	}
	reposCmd.Flags().Bool("all", false, "List all repositories accessible to the logged in user.")
	reposCmd.Flags().Bool("shared", false, "List all repositories that the user is a member of (excluding own repositories).")
	reposCmd.Flags().Bool("json", false, "Print listing in JSON format.")
	rootCmd.AddCommand(reposCmd)

	// Keys
	var keysCmd = &cobra.Command{
		Use:   "keys [--add <filename> | --delete <keynum> | --verbose | -v]",
		Short: "List, add, or delete public keys on the GIN services",
		Long:  w.Wrap("List, add, or delete SSH keys. If No argument is provided, a numbered list of key names is printed. The key number can be used with the '--delete' flag to remove a key from the server.\n\nThe command can also be used to add a public key to your account from an existing filename (see '--add' flag).", 80),
		Args:  cobra.MaximumNArgs(1),
		Run:   keys,
		DisableFlagsInUseLine: true,
	}
	keysCmd.Flags().String("add", "", "Specify a filename which contains a public key to be added to the GIN server.")
	keysCmd.Flags().Int("delete", 0, "Specify a number to delete the corresponding key from the server. Use 'gin keys' to get the numbered listing of keys.")
	keysCmd.Flags().BoolP("verbose", "v", false, "Verbose printing. Prints the entire public key.")
	rootCmd.AddCommand(keysCmd)

	// git and annex passthrough (unlisted)
	var gitCmd = &cobra.Command{
		Use:   "git <cmd> [<args>]...",
		Short: "Run a 'git' command through the gin client",
		Long:  "",
		Args:  cobra.ArbitraryArgs,
		Run:   gitrun,
		DisableFlagsInUseLine: true,
		Hidden:                true,
		DisableFlagParsing:    true,
	}
	rootCmd.AddCommand(gitCmd)

	var annexCmd = &cobra.Command{
		Use:   "annex <cmd> [<args>]...",
		Short: "Run a 'git annex' command through the gin client",
		Long:  "",
		Args:  cobra.ArbitraryArgs,
		Run:   annexrun,
		DisableFlagsInUseLine: true,
		Hidden:                true,
		DisableFlagParsing:    true,
	}
	rootCmd.AddCommand(annexCmd)

	rootCmd.Execute()

	util.LogWrite("EXIT OK")
}
