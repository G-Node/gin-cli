package main

// TODO: Nicer error handling. Print useful, descriptive messages.

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
	"time"

	ginclient "github.com/G-Node/gin-cli/gin-client"
	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/docopt/docopt-go"
	"github.com/howeyc/gopass"
)

var version string
var build string
var commit string
var verstr string

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
	gincl := web.NewClient("") // host configuration unnecessary
	err := gincl.LoadToken()
	if err != nil {
		util.Die("You are not logged in.")
	}

	err = web.DeleteToken()
	util.CheckErrorMsg(err, "Error deleting user token.")
	util.LogWrite("Logged out. Token deleted.")
	fmt.Println("You have been logged out.")
}

func createRepo(args []string) {
	var repoName, repoDesc string

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
	fmt.Printf("Creating repository '%s'...", repoPath)
	err = gincl.CreateRepo(repoName, repoDesc)
	// Parse error message and make error nicer
	util.CheckError(err)
	fmt.Println(" done.")

	// Clone repository after creation
	getRepo([]string{repoPath})
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

	gincl := ginclient.NewClient(util.Config.GinHost)
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	repoDir, err := gincl.CloneRepo(repostr)
	util.CheckError(err)

	ginclient.Workingdir = repoDir
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

	fmt.Print("Uploading...")

	err = gincl.Upload(args)
	util.CheckError(err)
	fmt.Println("done!")
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
	fmt.Print("Downloading...")
	err = gincl.DownloadRepo(content)
	fmt.Println("done!")
	util.CheckError(err)
}

func getContent(args []string) {
	if !ginclient.IsRepo() {
		util.Die("This command must be run from inside a gin repository.")
	}

	gincl := ginclient.NewClient(util.Config.GinHost)
	gincl.GitHost = util.Config.GitHost
	gincl.GitUser = util.Config.GitUser
	err := gincl.GetContent(args)
	util.CheckError(err)
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
	err = gincl.RmContent(args)
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

func init() {
	if version == "" {
		verstr = "GIN command line client [dev build]"
	} else {
		verstr = fmt.Sprintf("GIN command line client %s Build %s (%s)", version, build, commit)
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
	default:
		util.Die(usage)
	}
}
