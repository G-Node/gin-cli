package main

// TODO: Nicer error handling. Print useful, descriptive messages.

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/G-Node/gin-cli/auth"
	"github.com/G-Node/gin-cli/repo"
	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/docopt/docopt-go"
	"github.com/howeyc/gopass"
)

var version string
var build string
var commit string

const usage = `
GIN command line client

Usage: gin <command> [<args>...]
       gin --help
       gin --version

Options:
    -h --help    This help screen
    --version    Client version

Commands:
    login    [<username>]
    logout
    create   [<name>] [<description>]
    get      <repopath>
    upload
    download
    repos    [<username>]
    info     [<username>]
    keys     [-v | --verbose]
    keys     --add <filename>
    help     <command>

Use 'help' followed by a command to see full description of the command.
`

var cmdHelp = map[string]string{
	// LOGIN HELP
	"login": `USAGE

	gin login [<username>]

DESCRIPTION

	Login to the GIN services

ARGUMENTS

	<username>
		If no username is specified on the command line, you will be
		prompted for it. The login command always prompts for a
		password.`,

	"logout": `USAGE

	gin logout

DESCRIPTION

	Logout of the GIN services

	This command takes no arguments.`,

	// CREATE HELP
	"create": `USAGE

	gin create [<name>] [<description>]

DESCRIPTION

	Create a new repository on the GIN server.

ARGUMENTS

	<name>
		The name of the repository. If no <name> is provided, you will be
		prompted for one.  Specifying a name is required if a description is to
		be specified. Names should contain only alphanumeric characters, '-',
		and '_'.

	<description>
		A repository description (optional). The description should be
		specified as a single argument.  For most shells, this means the
		description should be in quotes.

EXAMPLES

	Create a repository. Prompt for name

		$ gin create

	Create a repository named 'example' with a description

		$ gin create example "An example repository"

	Create a repository named 'example' with no description

		$ gin create example
		`,

	// GET HELP
	"get": `USAGE

	gin get <repopath>

DESCRIPTION

	Download a remote repository to a new directory and initialise the directory
	with the default options. The local directory is referred to as the 'clone' of
	the repository.

ARGUMENTS

	<repopath>
		The repository path <repopath> must be specified on the command line.
		A repository path is the owner's username, followed by a "/" and the
		repository name.

EXAMPLES

	Get and intialise the repository named 'example' owned by user 'alice'

		$ gin get alice/example

	Get and initialise the repository named 'eegdata' owned by user 'peter'

		$ gin get peter/eegdata
		`,

	"upload": `USAGE

	gin upload

DESCRIPTION

	Upload changes made in a local repository clone to the remote repository on the
	GIN server.  This command must be called from within the (cloned) repository
	directory.  All changes made will be sent to the server, including addition of
	new files, modifications and renaming of existing files, and file deletions.

	This command takes no arguments.`,

	"download": `USAGE

	gin download

DESCRIPTION

	Download changes made in the remote repository on the GIN server to the local
	repository clone.  This command must be called from within the (cloned)
	repository directory. All changes made on the remote server will be retrieved,
	including addition of new files, modifications and renaming of existing files,
	and file deletions.

	This command takes no arguments.`,

	"repos": `USAGE

	gin repos [<username>]


DESCRIPTION

	List repositories on the server that provide read access. If no argument is
	provided, it will list all publicly accessible repositories on the GIN server.

ARGUMENTS

	<username>
		The name of the user whose repositories should be listed. This can be
		the username of the currently logged in user (YOU), in which case the
		command will list all repositories owned by YOU.  If it is the username
		of a different user, it will list all the repositories owned by the
		specified user that YOU have access to. This consists of public
		repositories and repositories shared with YOU.

	   `,

	//              Without any argument, lists all the publicly accessible
	//              repositories on the GIN server.
	//              If a <username> is specified, this command will list the
	//              specified user's publicly accessible repositories.
	//              If you are logged in, it will also list any repositories
	//              owned by the user that you have access to.

	"info": `USAGE

	gin info [<username>]

DESCRIPTION

	Print user information. If no argument is provided, it will print the
	information of the currently logged in user.

ARGUMENTS

	<username>
		The name of the user whose information should be printed. This can be
		the username of the currently logged in user, in which case the command
		will print all the profile information with indicators for which data
		is publicly visible. If it is the username of a different user, only
		the publicly visible information is printed.

`,

	"keys": `USAGE

	gin keys [-v | --verbose]
	gin keys --add <filename>

DESCRIPTION

	List or add SSH keys. If no argument is provided, it will list the
	description and fingerprint for each key associated with the logged in
	account.

	The command can also be used to add a public key to your account from an
	existing filename (see --add argument).

ARGUMENTS

	--verbose, -v
		Verbose printing. Prints the entire public key when listing.

	--add <filename>
		Specify a filename which contains a public key to be added to the GIN
		server.

EXAMPLES 

	Add a public key to your account, as generated from the default ssh-keygen 
	command

		$ gin keys --add ~/.ssh/id_rsa.pub
`,
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

	authcl := auth.NewClient(util.Config.AuthHost)
	err = authcl.Login(username, password, "gin-cli", "97196a1c-silly-biscuit3-d161ea15a676")
	util.CheckError(err)
	info, err := authcl.RequestAccount(username)
	util.CheckError(err)
	fmt.Printf("Hello %s. You are now logged in.\n", info.FirstName)
	// fmt.Printf("[Login success] You are now logged in as %s\n", username)
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
	util.LogWrite("Logged out. Token deleted.")
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
	repocl := repo.NewClient(util.Config.RepoHost)
	repocl.GitUser = util.Config.GitUser
	repocl.GitHost = util.Config.GitHost
	repocl.KeyHost = util.Config.AuthHost
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
	repocl := repo.NewClient(util.Config.RepoHost)
	repocl.GitUser = util.Config.GitUser
	repocl.GitHost = util.Config.GitHost
	repocl.KeyHost = util.Config.AuthHost
	err := repocl.CloneRepo(repostr)
	util.CheckError(err)
}

func upload(args []string) {
	if len(args) > 0 {
		util.Die(usage)
	}
	repocl := repo.NewClient(util.Config.RepoHost)
	repocl.GitUser = util.Config.GitUser
	repocl.GitHost = util.Config.GitHost
	repocl.KeyHost = util.Config.AuthHost
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
	repocl := repo.NewClient(util.Config.RepoHost)
	repocl.GitUser = util.Config.GitUser
	repocl.GitHost = util.Config.GitHost
	repocl.KeyHost = util.Config.AuthHost
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

	authcl := auth.NewClient(util.Config.AuthHost)
	keys, err := authcl.GetUserKeys()
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
	authcl := auth.NewClient(util.Config.AuthHost)
	err := authcl.LoadToken()
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
		description = fmt.Sprintf("%s@%s", authcl.Username, strconv.FormatInt(time.Now().Unix(), 10))
	}

	err = authcl.AddKey(string(keyBytes), description, false)
	util.CheckError(err)
	fmt.Printf("New key added '%s'\n", description)
}

func printAccountInfo(args []string) {
	var username string

	authcl := auth.NewClient(util.Config.AuthHost)
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
	repocl := repo.NewClient(util.Config.RepoHost)

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

func main() {
	verstr := fmt.Sprintf("GIN command line client %s Build %s (%s)", version, build, commit)

	args, _ := docopt.Parse(usage, nil, true, verstr, true)
	command := args["<command>"].(string)
	cmdArgs := args["<args>"].([]string)

	err := util.LogInit()
	util.CheckError(err)
	defer util.LogClose()

	err = util.LoadConfig()
	util.CheckError(err)

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
	case "help":
		help(cmdArgs)
	default:
		util.Die(usage)
	}
}
