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
	"github.com/docopt/docopt-go"
	"github.com/fatih/color"
	gogs "github.com/gogits/go-gogs-client"
	version "github.com/hashicorp/go-version"
	"github.com/howeyc/gopass"
)

var gincliversion string
var build string
var commit string
var verstr string
var minAnnexVersion = "6.20160126" // Introduction of git-annex add --json

var jsonflag = "--json"

var green = color.New(color.FgGreen).SprintFunc()
var red = color.New(color.FgRed).SprintFunc()

// requirelogin prompts for login if the user is not already logged in.
// It only checks if a local token exists and does not confirm its validity with the server.
// The function should be called at the start of any command that requires being logged in to run.
func requirelogin(gincl *ginclient.Client, prompt bool) {
	err := gincl.LoadToken()
	if prompt {
		if err != nil {
			login([]string{})
		}
		err = gincl.LoadToken()
	}
	util.CheckError(err)
}

// checkJSON checks if the JSON flag is in the first position of the arg list and either true followed
// by the rest of the arguments or false with the original argument list.
func checkJSON(args []string) (json bool, rargs []string) {
	rargs = args
	if len(args) > 0 {
		if args[0] == jsonflag {
			json = true
			rargs = args[1:]
		}
	}
	return
}

// login requests credentials, performs login with auth server, and stores the token.
func login(args []string) {
	var username string
	var password string

	if len(args) == 0 {
		// prompt for login
		fmt.Print("Login: ")
		fmt.Scanln(&username)
	} else if len(args) > 1 {
		util.Die(usage)
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

func logout(args []string) {
	if len(args) > 0 {
		util.Die(usage)
	}
	gincl := ginclient.NewClient(util.Config.GinHost)
	err := gincl.LoadToken()
	if err != nil {
		util.Die("You are not logged in.")
	}

	gincl.Logout()
	fmt.Println("You have been logged out.")
}

func createRepo(args []string) {
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(gincl, true)

	var repoName, repoDesc string
	var here, noclone bool

	if len(args) > 0 {
		if args[0] == "--here" {
			here = true
			args = args[1:]
		} else if args[0] == "--no-clone" {
			noclone = true
			args = args[1:]
		}
	}

	if len(args) == 0 {
		fmt.Print("Repository name: ")
		fmt.Scanln(&repoName)
	} else if len(args) > 2 {
		util.Die(usage)
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
		getRepo([]string{repoPath})
	}
}

func deleteRepo(args []string) {
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(gincl, true)
	var repostr, confirmation string

	if len(args) == 0 {
		util.Die(usage)
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

func getRepo(args []string) {
	jsonout, args := checkJSON(args)
	var repostr string
	if len(args) != 1 {
		util.Die(usage)
	} else {
		repostr = args[0]
	}

	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(gincl, !jsonout)

	if !isValidRepoPath(repostr) {
		util.Die(fmt.Sprintf("Invalid repository path '%s'. Full repository name should be the owner's username followed by the repository name, separated by a '/'.\nType 'gin help get' for information and examples.", repostr))
	}

	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	clonechan := make(chan ginclient.RepoFileStatus)
	go gincl.CloneRepo(repostr, clonechan)
	printProgress(clonechan, jsonout)
}

func lsRepo(args []string) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	var short bool
	jsonout, args := checkJSON(args)
	if len(args) > 0 && (args[0] == "-s" || args[0] == "--short") {
		short = true
		args = args[1:]
	}

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

func lock(args []string) {
	jsonout, args := checkJSON(args)
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	gincl := ginclient.NewClient(util.Config.GinHost)
	lockchan := make(chan ginclient.RepoFileStatus)
	go gincl.LockContent(args, lockchan)
	printProgress(lockchan, jsonout)
}

func unlock(args []string) {
	jsonout, args := checkJSON(args)
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
	prevlinelength := 0 // used to clear lines before overwriting
	nerrors := 0
	for stat := range statuschan {
		var msgparts []string
		if jsonout {
			j, _ := json.Marshal(stat)
			fmt.Println(string(j))
			continue
		}
		if stat.FileName != fname || stat.State != state {
			// New line if new file or new state
			if fname != "" {
				fmt.Println()
				prevlinelength = 0
			}
			fname = stat.FileName
			state = stat.State
		}
		msgparts = append(msgparts, stat.State, stat.FileName)
		if stat.Err == nil {
			if stat.Progress == "100%" {
				msgparts = append(msgparts, green("OK"))
			} else {
				msgparts = append(msgparts, stat.Progress, stat.Rate)
			}
		} else {
			msgparts = append(msgparts, red(stat.Err.Error()))
			nerrors++
		}
		fmt.Printf("\r%s", strings.Repeat(" ", prevlinelength)) // clear the previous line
		prevlinelength, _ = fmt.Fprintf(color.Output, "\r%s", util.CleanSpaces(strings.Join(msgparts, " ")))
	}
	if prevlinelength > 0 {
		fmt.Println()
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

func upload(args []string) {
	jsonout, args := checkJSON(args)
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(gincl, !jsonout)
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

func download(args []string) {
	jsonout, args := checkJSON(args)
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(gincl, !jsonout)
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	var content bool
	if len(args) > 0 {
		if args[0] != "--content" {
			util.Die(usage)
		}
		content = true
	}

	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	lockchan := make(chan ginclient.RepoFileStatus)
	go gincl.LockContent([]string{}, lockchan)
	printProgress(lockchan, jsonout)
	dlchan := make(chan ginclient.RepoFileStatus)
	if !content && !jsonout {
		fmt.Print("Downloading...")
	}
	go gincl.Download(content, dlchan)
	printProgress(dlchan, jsonout)
	if !content && !jsonout {
		fmt.Fprintln(color.Output, green("OK"))
	}
}

func getContent(args []string) {
	jsonout, args := checkJSON(args)
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(gincl, !jsonout)
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	getcchan := make(chan ginclient.RepoFileStatus)
	go gincl.GetContent(args, getcchan)
	printProgress(getcchan, jsonout)
}

func remove(args []string) {
	jsonout, args := checkJSON(args)
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(gincl, !jsonout)
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

func keys(args []string) {
	if len(args) > 0 {
		subcommand := args[0]
		if subcommand == "--add" {
			addKey(args)
		} else if subcommand == "--delete" {
			delKey(args)
		} else {
			util.Die(usage)
		}
	} else {
		printKeys(args)
	}
}

func printKeys(args []string) {
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(gincl, true)
	printFull := false
	if len(args) > 1 {
		util.Die(usage)
	} else if len(args) == 1 {
		if args[0] == "-v" || args[0] == "--verbose" {
			printFull = true
		} else {
			util.Die(usage)
		}
	}

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
		if printFull {
			fmt.Printf("--- Key ---\n%s\n", key.Key)
		}
	}
}

func addKey(args []string) {
	if len(args) != 2 {
		util.Die(usage)
	}
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(gincl, true)
	err := gincl.LoadToken()
	util.CheckError(err)

	filename := args[1]

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

func delKey(args []string) {
	if len(args) != 2 {
		util.Die(usage)
	}

	idx, err := strconv.Atoi(args[1])
	if err != nil {
		util.Die(usage)
	}
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(gincl, true)
	err = gincl.LoadToken()
	util.CheckError(err)

	name, err := gincl.DeletePubKeyByIdx(idx)
	util.CheckError(err)
	fmt.Printf("Deleted key with name '%s'\n", name)
}

func printAccountInfo(args []string) {
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

func repos(args []string) {
	jsonout, args := checkJSON(args)
	var allrepos, sharedrepos bool
	if len(args) > 0 {
		if args[0] == "--all" {
			allrepos = true
			args = args[1:]
		} else if args[0] == "--shared" {
			sharedrepos = true
			args = args[1:]
		}
	}
	if (allrepos || sharedrepos) && len(args) > 0 {
		util.Die(usage)
	}
	if len(args) > 1 {
		util.Die(usage)
	}
	gincl := ginclient.NewClient(util.Config.GinHost)
	requirelogin(gincl, true)
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

func help(args []string) {
	if len(args) != 1 {
		util.Die(usage)
	}
	helptext, ok := cmdHelp[args[0]]
	if !ok {
		util.Die(usage)
	}

	fmt.Println(helptext)
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

func gitrun(args []string) {
	gincl := ginclient.NewClient(util.Config.GinHost)
	err := gincl.LoadToken()
	util.CheckError(err)

	cmd, err := ginclient.RunGitCommand(args...)
	util.CheckError(err)
	for {
		line, readerr := cmd.OutPipe.ReadLine()
		if readerr != nil {
			break
		}
		fmt.Print(line)
	}
	fmt.Print(cmd.ErrPipe.ReadAll())
	err = cmd.Wait()
	if err != nil {
		os.Exit(1)
	}
}

func annexrun(args []string) {
	gincl := ginclient.NewClient(util.Config.GinHost)
	err := gincl.LoadToken()
	util.CheckError(err)
	cmd, err := ginclient.RunAnnexCommand(args...)
	util.CheckError(err)
	var line string
	for {
		line, err = cmd.OutPipe.ReadLine()
		if err != nil {
			break
		}
		fmt.Print(line)
	}
	fmt.Print(cmd.ErrPipe.ReadAll())
	err = cmd.Wait()
	if err != nil {
		os.Exit(1)
	}
}

func printversion(args []string) {
	jsonout, args := checkJSON(args)
	if len(args) > 0 {
		util.Die(usage)
	}
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
	args, _ := docopt.Parse(usage, nil, true, verstr, true)
	command := args["<command>"].(string)
	cmdArgs := args["<args>"].([]string)

	err := util.LogInit()
	util.CheckError(err)
	defer util.LogClose()

	util.LogWrite("COMMAND: %s %s", command, strings.Join(cmdArgs, " "))

	err = util.LoadConfig()
	util.CheckError(err)
	checkAnnexVersion()

	switch command {
	case "login":
		login(cmdArgs)
	case "create":
		createRepo(cmdArgs)
	case "delete":
		deleteRepo(cmdArgs)
	case "get":
		getRepo(cmdArgs)
	case "ls":
		lsRepo(cmdArgs)
	case "unlock":
		unlock(cmdArgs)
	case "lock":
		lock(cmdArgs)
	case "upload":
		upload(cmdArgs)
	case "download":
		download(cmdArgs)
	case "get-content":
		getContent(cmdArgs)
	case "getc":
		getContent(cmdArgs)
	case "remove-content":
		remove(cmdArgs)
	case "rmc":
		remove(cmdArgs)
	case "info":
		printAccountInfo(cmdArgs)
	case "keys":
		keys(cmdArgs)
	case "repos":
		repos(cmdArgs)
	case "logout":
		logout(cmdArgs)
	case "help":
		help(cmdArgs)
	case "git":
		gitrun(cmdArgs)
	case "annex":
		annexrun(cmdArgs)
	case "version":
		printversion(cmdArgs)
	default:
		util.Die(usage)
	}
}
