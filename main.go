package main

// TODO: Nicer error handling. Print useful, descriptive messages.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/G-Node/gin-cli/auth"
	"github.com/G-Node/gin-cli/repo"
	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/G-Node/gin-core/gin"
	"github.com/docopt/docopt-go"
	"github.com/howeyc/gopass"
)

// TODO: Load from config
const authhost = "https://auth.gin.g-node.org"
const repohost = "https://repo.gin.g-node.org"
const githost = "gin.g-node.org"
const gituser = "git"

// Login Request credentials, perform login, and store token
func login(userarg interface{}) {

	var username, password string

	if userarg == nil {
		// prompt for login
		fmt.Print("Login: ")
		fmt.Scanln(&username)
	} else {
		username = userarg.(string)
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

	params := gin.LoginRequest{
		Scope:        "repo-read repo-write account-read account-write",
		Username:     username,
		Password:     password,
		GrantType:    "password",
		ClientID:     "gin-cli",
		ClientSecret: "97196a1c-silly-biscuit3-d161ea15a676",
	}

	authcl := auth.NewClient(authhost)
	res, err := authcl.Post("/oauth/token", params)
	util.CheckError(err)

	defer web.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	util.CheckError(err)

	var authresp gin.TokenResponse
	err = json.Unmarshal(b, &authresp)
	util.CheckError(err)

	tokenfile := filepath.Join(util.ConfigPath(), "token")
	err = ioutil.WriteFile(tokenfile, []byte(authresp.AccessToken), 0600)
	// util.CheckErrorMsg(err, "[Error] Login failed while storing token.")
	util.CheckError(err)
	fmt.Printf("[Login success] You are now logged in as %s\n", username)
	// fmt.Printf("You have been granted the following permissions: %v\n", strings.Replace(authresp.Scope, " ", ", ", -1))
}

func createRepo(name, description interface{}) {
	var repoName string
	if name == nil || name.(string) == "" {
		fmt.Print("Repository name: ")
		repoName = ""
		fmt.Scanln(&repoName)
	} else {
		repoName = name.(string)
	}
	// TODO: Check name validity before sending to server?
	repoDesc := ""
	if description != nil {
		repoDesc = description.(string)
	}
	repo.CreateRepo(repoName, repoDesc)
}

func getRepo(repoarg interface{}) {
	var repostr string
	if repoarg != nil {
		repostr = repoarg.(string)
		repo.CloneRepo(repostr)
	} else {
		util.Die("No repository specified.")
	}
}

func upload() {
	repo.UploadRepo(".")
}

func download() {
	if repo.IsRepo(".") {
		repo.DownloadRepo()
	} else {
		util.Die("Current directory is not a repository.")
	}
}

// condAppend Conditionally append str to b if not empty
func condAppend(b *bytes.Buffer, str *string) {
	if str != nil && *str != "" {
		_, _ = b.WriteString(*str + " ")
	}
}

func printKeys(printFull bool) {

	keys := auth.GetUserKeys()
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

func addKey(fnarg interface{}) {
	noargError := fmt.Errorf("Please specify a public key file.\n")
	if fnarg == nil {
		util.CheckError(noargError)
	}

	filename := fnarg.(string)
	if filename == "" {
		util.CheckError(noargError)
	}

	keyBytes, err := ioutil.ReadFile(filename)
	util.CheckError(err)
	// TODO: Accept custom description for key and simply default to filename
	key := string(keyBytes)
	description := strings.TrimSpace(strings.Split(key, " ")[2])
	err = auth.AddKey(string(keyBytes), description, false)
	util.CheckError(err)
	fmt.Printf("New key added '%s'\n", description)
}

func printAccountInfo(userarg interface{}) {
	var username string
	currentUser, token, _ := auth.LoadToken(true)

	if userarg == nil {
		username = currentUser
	} else {
		username = userarg.(string)
	}

	if username == "" {
		// prompt for username
		fmt.Print("Specify username for info lookup: ")
		username = ""
		fmt.Scanln(&username)
	}

	info := auth.RequestAccount(username, token)

	var fullnameBuffer bytes.Buffer

	condAppend(&fullnameBuffer, info.Title)
	condAppend(&fullnameBuffer, &info.FirstName)
	condAppend(&fullnameBuffer, info.MiddleName)
	condAppend(&fullnameBuffer, &info.LastName)

	var outBuffer bytes.Buffer

	_, _ = outBuffer.WriteString(fmt.Sprintf("User [%s]\nName: %s\n", info.Login, fullnameBuffer.String()))

	if info.Email != nil && info.Email.Email != "" {
		_, _ = outBuffer.WriteString(fmt.Sprintf("Email: %s\n", info.Email.Email))
		// TODO: Display public status if current user == info.Login
	}

	if info.Affiliation != nil {
		var affiliationBuffer bytes.Buffer
		affiliation := info.Affiliation

		condAppend(&affiliationBuffer, &affiliation.Department)
		condAppend(&affiliationBuffer, &affiliation.Institute)
		condAppend(&affiliationBuffer, &affiliation.City)
		condAppend(&affiliationBuffer, &affiliation.Country)

		if affiliationBuffer.Len() > 0 {
			_, _ = outBuffer.WriteString(fmt.Sprintf("Affiliation: %s\n", affiliationBuffer.String()))
		}
	}

	fmt.Println(outBuffer.String())
}

func listUserRepos(userarg interface{}) {
	var username string
	currentUser, token, _ := auth.LoadToken(false)

	if currentUser == "" {
		util.CheckError(fmt.Errorf("This command requires login."))
	}

	if userarg == nil {
		username = currentUser
	} else {
		username = userarg.(string)
	}

	info := repo.GetRepos(username, token)

	fmt.Printf("Repositories owned by %s\n", username)
	for idx, r := range info {
		fmt.Printf("%d:  %s\n", idx+1, r.Name)
	}
}

func listPubRepos() {
	repos := repo.GetRepos("", "")
	fmt.Printf("Public repositories\n")
	for idx, repoInfo := range repos {
		fmt.Printf("%d: %s [head: %s]\n", idx+1, repoInfo.Name, repoInfo.Head)
		fmt.Printf(" - %s\n", repoInfo.Description)
	}
}

func main() {
	usage := `
GIN command line client

Usage:
	gin login    [<username>]
	gin create   [<name>] [-d <description>]
	gin get      [<repopath>]
	gin upload
	gin download
	gin repos    [<username>]
	gin info     [<username>]
	gin keys     [-v | --verbose]
	gin keys     --add <filename>
	gin public
`

	args, _ := docopt.Parse(usage, nil, true, "gin cli 0.0", false)

	switch {
	case args["login"].(bool):
		login(args["<username>"])
	case args["create"].(bool):
		createRepo(args["<name>"], args["<description>"])
	case args["get"].(bool):
		getRepo(args["<repopath>"])
	case args["upload"].(bool):
		upload()
	case args["download"].(bool):
		download()
	case args["info"].(bool):
		printAccountInfo(args["<username>"])
	case args["keys"].(bool):
		if args["--add"].(bool) {
			addKey(args["<filename>"])
		} else {
			printFullKeys := false
			if args["-v"].(bool) || args["--verbose"].(bool) {
				printFullKeys = true
			}
			printKeys(printFullKeys)
		}
	case args["repos"].(bool):
		listUserRepos(args["<username>"])
	case args["public"].(bool):
		listPubRepos()
	}
}
