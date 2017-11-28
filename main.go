package main

import (
	"bytes"
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
	version "github.com/hashicorp/go-version"
	"github.com/howeyc/gopass"
)

var gincliversion string
var build string
var commit string
var verstr string
var minAnnexVersion = "6.20160126" // Introduction of git-annex add --json

var green = color.New(color.FgGreen)

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
			util.Die("[Error] Input too long.")
		}
		util.Die(err.Error())
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
	var repoName, repoDesc string
	var here bool

	if len(args) > 0 && args[0] == "--here" {
		here = true
		args = args[1:]
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
	// TODO: Check name validity before sending to server?
	gincl := ginclient.NewClient(util.Config.GinHost)
	err := gincl.LoadToken()
	util.CheckError(err)

	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	repoPath := fmt.Sprintf("%s/%s", gincl.Username, repoName)
	fmt.Printf("Creating repository '%s'... ", repoPath)
	err = gincl.CreateRepo(repoName, repoDesc)
	// Parse error message and make error nicer
	util.CheckError(err)
	_, _ = green.Println("OK")

	if here {
		// Init cwd
		ginclient.Workingdir = "."
		fmt.Printf("Initialising local storage... ")
		_, err = gincl.InitDir(repoPath)
		util.CheckError(err)
		_, _ = green.Println("OK")
	} else {
		// Clone repository after creation
		getRepo([]string{repoPath})
	}
}

func deleteRepo(args []string) {
	var repostr, confirmation string

	if len(args) == 0 {
		util.Die(usage)
	} else {
		repostr = args[0]
	}

	gincl := ginclient.NewClient(util.Config.GinHost)
	err := gincl.LoadToken()
	util.CheckError(err)

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
	var repostr string
	if len(args) != 1 {
		util.Die(usage)
	} else {
		repostr = args[0]
	}

	if !isValidRepoPath(repostr) {
		util.Die(fmt.Sprintf("Invalid repository path '%s'. Full repository name should be the owner's username followed by the repository name, separated by a '/'.\nType 'gin help get' for information and examples.", repostr))
	}

	fmt.Printf("Fetching repository '%s'... ", repostr)
	gincl := ginclient.NewClient(util.Config.GinHost)
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	_, err := gincl.CloneRepo(repostr)
	util.CheckError(err)
	_, _ = green.Println("OK")
}

func lsRepo(args []string) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	var short bool
	for idx, arg := range args {
		if arg == "-s" || arg == "--short" {
			args = append(args[:idx], args[idx+1:]...)
			short = true
			break
		}
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
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	err := ginclient.AnnexLock(args...)
	util.CheckError(err)
}

func unlock(args []string) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	err := ginclient.AnnexUnlock(args...)
	util.CheckError(err)
}

func printProgress(statuschan chan ginclient.RepoFileStatus) {
	var fname string
	prevlinelength := 0 // used to clear lines before overwriting
	for {
		stat, ok := <-statuschan
		if !ok {
			break
		}
		// TODO: Parse error
		util.CheckError(stat.Err)
		if stat.FileName != fname {
			// New line if new file status
			if fname != "" {
				fmt.Println()
				prevlinelength = 0
			}
			fname = stat.FileName
		}
		progress := stat.Progress
		rate := stat.Rate
		if len(rate) > 0 {
			rate = fmt.Sprintf("(%s)", rate)
		}
		if progress == "100%" || stat.State == "Added" {
			progress = green.Sprint("OK")
		}
		fmt.Printf("\r%s", strings.Repeat(" ", prevlinelength)) // clear the previous line
		prevlinelength, _ = fmt.Printf("\r%s %s: %s %s", stat.State, fname, progress, rate)
	}
	fmt.Println()
}

func upload(args []string) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	err := ginclient.AnnexLock(args...)
	util.CheckError(err)

	gincl := ginclient.NewClient(util.Config.GinHost)
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser

	if len(args) == 0 {
		fmt.Println("No files specified for upload. Synchronising metadata.")
		fmt.Printf("To upload all files under the current directory, use:\n\n\tgin upload .\n\n")
	}

	uploadchan := make(chan ginclient.RepoFileStatus)
	go gincl.Upload(args, uploadchan)
	printProgress(uploadchan)
}

func download(args []string) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	err := ginclient.AnnexLock()
	util.CheckError(err)

	var content bool
	if len(args) > 0 {
		if args[0] != "--content" {
			util.Die(usage)
		}
		content = true
	}

	gincl := ginclient.NewClient(util.Config.GinHost)
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	dlchan := make(chan ginclient.RepoFileStatus)
	if !content {
		fmt.Print("Downloading...")
	}
	go gincl.Download(content, dlchan)
	printProgress(dlchan)
	if !content {
		_, _ = green.Println("OK")
	}
}

func getContent(args []string) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	gincl := ginclient.NewClient(util.Config.GinHost)
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	getcchan := make(chan ginclient.RepoFileStatus)
	go gincl.GetContent(args, getcchan)
	printProgress(getcchan)
}

func remove(args []string) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}
	err := ginclient.AnnexLock(args...)
	util.CheckError(err)

	gincl := ginclient.NewClient(util.Config.GinHost)
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	err = ginclient.AnnexDrop(args)
	util.CheckError(err)
}

func keys(args []string) {
	if len(args) > 0 && args[0] == "--add" {
		addKey(args)
	} else {
		printKeys(args)
	}
}

func printKeys(args []string) {
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

	gincl := ginclient.NewClient(util.Config.GinHost)
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
	err := gincl.LoadToken()
	if err != nil {
		util.Die("This command requires login.")
	}

	filename := args[1]

	keyBytes, err := ioutil.ReadFile(filename)
	util.CheckError(err)
	// TODO: Accept custom description for key and simply default to filename
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

func printAccountInfo(args []string) {
	var username string

	gincl := ginclient.NewClient(util.Config.GinHost)
	_ = gincl.LoadToken()

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

func repos(args []string) {
	if len(args) > 1 {
		util.Die(usage)
	}
	var arg string
	gincl := ginclient.NewClient(util.Config.GinHost)
	err := gincl.LoadToken()
	if len(args) == 0 {
		if err == nil {
			arg = gincl.Username
		}
	} else {
		arg = args[0]
		if arg == "-p" {
			arg = "--public"
		} else if arg == "-s" {
			arg = "--shared-with-me"
		}
	}
	repolist, err := gincl.ListRepos(arg)
	util.CheckError(err)

	if arg == "" || arg == "--public" {
		fmt.Print("Listing all public repositories:\n\n")
	} else if arg == "--shared-with-me" {
		fmt.Print("Listing all accessible shared repositories:\n\n")
	} else {
		if gincl.Username == "" {
			fmt.Printf("You are not logged in.\nListing only public repositories owned by '%s':\n\n", arg)
		} else if arg == gincl.Username {
			fmt.Print("Listing your repositories:\n\n")
		} else {
			fmt.Printf("Listing accessible repositories owned by '%s':\n\n", arg)
		}
	}
	for idx, repoInfo := range repolist {
		fmt.Printf("%d: %s\n", idx+1, repoInfo.FullName)
		fmt.Printf("Description: %s\n", strings.Trim(repoInfo.Description, "\n"))
		if !repoInfo.Private {
			fmt.Println("[This repository is public]")
		}
		fmt.Println()
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

func checkAnnexVersion(verstring string) {
	errmsg := fmt.Sprintf("The GIN Client requires git-annex %s or newer", minAnnexVersion)
	systemver, err := version.NewVersion(verstring)
	if err != nil {
		util.Die(fmt.Sprintln("git-annex not found") + errmsg)
	}
	minver, _ := version.NewVersion(minAnnexVersion)
	if systemver.LessThan(minver) {
		util.Die(errmsg + fmt.Sprintf("Found version %s", systemver))
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

	annexver, err := ginclient.GetAnnexVersion()
	util.CheckError(err)
	checkAnnexVersion(annexver)

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
	default:
		util.Die(usage)
	}
}
