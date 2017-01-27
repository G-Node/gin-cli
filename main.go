package main

// TODO: Nicer error handling. Print useful, descriptive messages.

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/G-Node/gin-cli/auth"
	"github.com/G-Node/gin-cli/repo"
	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/docopt/docopt-go"
	"github.com/howeyc/gopass"
)

const usage = `
GIN command line client

Usage: gin [--version] <command> [<args>...]

Options:
    -h --help    This help screen
    --version    Client version

Commands:
    gin login    [<username>]
    gin logout
    gin create   [<name>] [<description>]
    gin get      <repopath>
    gin upload
    gin download
    gin repos    [<username>]
    gin info     [<username>]
    gin keys     [-v | --verbose]
    gin keys     --add <filename>
`

const cmdUsage = `
Command help:

    login        Login to the GIN services

                 If no <username> is specified on the command line, you will be
                 prompted for it. The login command always prompts for a
                 password.


    logout       Logout of the GIN services


    create       Create a new repository on the GIN server

                 If no <name> is provided, you will be prompted for one.
                 A repository <description> can only be specified on the
                 command line after the <name>.
                 Login required.


    get          Download a remote repository to a new directory

                 The repository path <repopath> must be specified on the
                 command line. A repository path is the owner's username,
                 followed by a "/" and the repository name
                 (e.g., peter/eegdata).
                 Login required.


    upload       Upload local repository changes to the remote repository

                 Uploads any changes made on the local data to the GIN server.
                 The upload command should be run from inside the directory of
                 an existing repository.


    download     Download remote repository changes to the local repository

                 Downloads any changes made to the data on the server to the
                 local data directory.
                 The download command should be run from inside the directory
                 of an existing repository.


    repos        List accessible repositories

                 Without any argument, lists all the publicly accessible
                 repositories on the GIN server.
                 If a <username> is specified, this command will list the
                 specified user's publicly accessible repositories.
                 If you are logged in, it will also list any repositories
                 owned by the user that you have access to.


    info         Print user information

                 Without argument, print the information of the currently
                 logged in user or, if you are not logged in, prompt for a
                 username to look up.
                 If a <username> is specified, print the user's information.


    keys         List or add SSH keys

                 By default will list the keys (description and fingerprint)
                 associated with the logged in user. The verbose flag will also
                 print the full public keys.
                 To add a new key, use the --add option and specify a pub key
                 <filename>.
                 Login required.
`

// TODO: Load from config
const authhost = "https://auth.gin.g-node.org"
const repohost = "https://repo.gin.g-node.org"
const githost = "gin.g-node.org"
const gituser = "git"

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
	password = ""
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

	authcl := auth.NewClient(authhost)
	err = authcl.Login(username, password, "gin-cli", "97196a1c-silly-biscuit3-d161ea15a676")
	util.CheckError(err)
	fmt.Printf("[Login success] You are now logged in as %s\n", username)
	// fmt.Printf("You have been granted the following permissions: %v\n", strings.Replace(authresp.Scope, " ", ", ", -1))
}

func logout(args []string) {
	if len(args) > 0 {
		util.Die(usage)
	}
	authcl := auth.NewClient("") // host configuration unnecessary
	err := authcl.LoadToken()
	if err != nil {
		util.Die("You are not logged in.")
	}

	err = web.DeleteToken()
	util.CheckErrorMsg(err, "Error deleting user token.")
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
	repocl := repo.NewClient(repohost)
	repocl.GitUser = gituser
	repocl.GitHost = githost
	repocl.KeyHost = authhost
	err := repocl.CreateRepo(repoName, repoDesc)
	util.CheckError(err)
}

func getRepo(args []string) {
	var repostr string
	if len(args) != 1 {
		util.Die(usage)
	} else {
		repostr = args[0]
	}
	repocl := repo.NewClient(repohost)
	repocl.GitUser = gituser
	repocl.GitHost = githost
	repocl.KeyHost = authhost
	err := repocl.CloneRepo(repostr)
	util.CheckError(err)
}

func upload(args []string) {
	if len(args) > 0 {
		util.Die(usage)
	}
	repocl := repo.NewClient(repohost)
	repocl.GitUser = gituser
	repocl.GitHost = githost
	repocl.KeyHost = authhost
	err := repocl.UploadRepo(".")
	util.CheckError(err)
}

func download(args []string) {
	if len(args) > 0 {
		util.Die(usage)
	}
	if !repo.IsRepo(".") {
		util.Die("Current directory is not a repository.")
	}
	repocl := repo.NewClient(repohost)
	repocl.GitUser = gituser
	repocl.GitHost = githost
	repocl.KeyHost = authhost
	err := repocl.DownloadRepo(".")
	util.CheckError(err)
}

// condAppend Conditionally append str to b if not empty
func condAppend(b *bytes.Buffer, str *string) {
	if str != nil && *str != "" {
		_, _ = b.WriteString(*str + " ")
	}
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

	authcl := auth.NewClient(authhost)
	keys, err := authcl.GetUserKeys()
	util.CheckError(err)

	nkeys := len(keys)
	var plural string
	if nkeys == 1 {
		plural = ""
	} else {
		plural = "s"
	}
	fmt.Printf("You have %d key%s associated with your account.\n\n", nkeys, plural)
	for idx, key := range keys {
		fmt.Printf("[%v] \"%s\" ", idx+1, key.Description)
		fmt.Printf("(Fingerprint: %s)\n", key.Fingerprint)
		if printFull {
			fmt.Printf("--- Key ---\n%s\n", key.Key)
		}
	}
}

func addKey(args []string) {
	if len(args) != 2 {
		util.Die(usage)
	}

	filename := args[1]

	keyBytes, err := ioutil.ReadFile(filename)
	util.CheckError(err)
	// TODO: Accept custom description for key and simply default to filename
	key := string(keyBytes)
	description := strings.TrimSpace(strings.Split(key, " ")[2])

	authcl := auth.NewClient(authhost)
	err = authcl.AddKey(string(keyBytes), description, false)
	util.CheckError(err)
	fmt.Printf("New key added '%s'\n", description)
}

func printAccountInfo(args []string) {
	var username string

	authcl := auth.NewClient(authhost)
	_ = authcl.LoadToken()

	if len(args) == 0 {
		username = authcl.Username
	} else {
		username = args[0]
	}

	if username == "" {
		// prompt for username
		fmt.Print("Specify username for info lookup: ")
		username = ""
		fmt.Scanln(&username)
	}

	info, err := authcl.RequestAccount(username)
	util.CheckError(err)

	var fullnameBuffer bytes.Buffer

	condAppend(&fullnameBuffer, info.Title)
	condAppend(&fullnameBuffer, &info.FirstName)
	condAppend(&fullnameBuffer, info.MiddleName)
	condAppend(&fullnameBuffer, &info.LastName)

	var outBuffer bytes.Buffer

	_, _ = outBuffer.WriteString(fmt.Sprintf("User [%s]\nName: %s\n", info.Login, fullnameBuffer.String()))

	if info.Email != nil && info.Email.Email != "" {
		_, _ = outBuffer.WriteString(fmt.Sprintf("Email: %s", info.Email.Email))
		if info.Email.IsPublic && info.Login == authcl.Username {
			_, _ = outBuffer.WriteString(fmt.Sprintf(" (publicly visible)"))
		}
		_, _ = outBuffer.WriteString(fmt.Sprintf("\n"))
	}

	if info.Affiliation != nil {
		var affiliationBuffer bytes.Buffer
		affiliation := info.Affiliation

		condAppend(&affiliationBuffer, &affiliation.Department)
		condAppend(&affiliationBuffer, &affiliation.Institute)
		condAppend(&affiliationBuffer, &affiliation.City)
		condAppend(&affiliationBuffer, &affiliation.Country)

		if affiliationBuffer.Len() > 0 {
			_, _ = outBuffer.WriteString(fmt.Sprintf("Affiliation: %s", affiliationBuffer.String()))
			if info.Affiliation.IsPublic && info.Login == authcl.Username {
				_, _ = outBuffer.WriteString(fmt.Sprintf(" (publicly visible)"))
			}
			_, _ = outBuffer.WriteString(fmt.Sprintf("\n"))
		}
	}

	fmt.Println(outBuffer.String())
}

func listRepos(args []string) {
	if len(args) > 1 {
		util.Die(usage)
	}
	var username string
	repocl := repo.NewClient(repohost)

	if len(args) == 0 {
		username = ""
	} else {
		username = args[0]
	}
	repos, err := repocl.GetRepos(username)
	util.CheckError(err)

	for idx, repoInfo := range repos {
		fmt.Printf("%d: %s/%s\n", idx+1, repoInfo.Owner, repoInfo.Name)
		fmt.Printf("Description: %s\n", strings.Trim(repoInfo.Description, "\n"))
		if repoInfo.Public {
			fmt.Println("[This repository is public]")
		}
		fmt.Println()
	}
}

func tree(args []string) {
	repo.PrintChanges(nil)
}

func main() {
	fullUsage := usage + "\n" + cmdUsage
	args, _ := docopt.Parse(fullUsage, nil, true, "GIN command line client v0.1", true)
	command := args["<command>"].(string)
	cmdArgs := args["<args>"].([]string)

	switch command {
	case "login":
		login(cmdArgs)
	case "create":
		createRepo(cmdArgs)
	case "get":
		getRepo(cmdArgs)
	case "upload":
		upload(cmdArgs)
	case "download":
		download(cmdArgs)
	case "info":
		printAccountInfo(cmdArgs)
	case "keys":
		keys(cmdArgs)
	case "repos":
		listRepos(cmdArgs)
	case "logout":
		logout(cmdArgs)
	case "status":
		tree(cmdArgs)
	default:
		util.Die(usage)
	}
}
