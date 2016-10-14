package main

// TODO: Nicer error handling. Print useful, descriptive messages.

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/G-Node/gin-cli/auth"
	"github.com/G-Node/gin-cli/repo"
	"github.com/docopt/docopt-go"
)

func createRepo(name, description interface{}) error {
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
	return repo.CreateRepo(repoName, repoDesc)
}

func upload(patharg interface{}) error {
	var pathstr string
	if patharg != nil {
		pathstr = patharg.(string)
	}
	return repo.UploadRepo(pathstr)
}

func download(patharg interface{}) error {
	var pathstr string
	if patharg != nil {
		pathstr = patharg.(string)
		return repo.CloneRepo(pathstr)
	}

	// No repo specified -- attempting to pull in cwd
	if repo.IsRepo(".") {
		return repo.DownloadRepo()
	}

	return fmt.Errorf("Current directory is not a repository.")
}

// condAppend Conditionally append str to b if not empty
func condAppend(b *bytes.Buffer, str *string) {
	if str != nil && *str != "" {
		_, _ = b.WriteString(*str + " ")
	}
}

func printKeys(printFull bool) error {

	keys, err := auth.GetUserKeys()
	if err != nil {
		return err
	}
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

	return err
}

func addKey(fnarg interface{}) error {
	noargError := fmt.Errorf("Please specify a public key file.\n")
	if fnarg == nil {
		return noargError
	}

	filename := fnarg.(string)
	if filename == "" {
		return noargError
	}

	keyBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	// TODO: Accept custom description for key and simply default to filename
	key := string(keyBytes)
	description := strings.TrimSpace(strings.Split(key, " ")[2])
	err = auth.AddKey(string(keyBytes), description)
	if err != nil {
		return err
	}
	fmt.Printf("New key added '%s'\n", description)
	return nil
}

func printAccountInfo(userarg interface{}) error {
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

	info, err := auth.RequestAccount(username, token)
	if err != nil {
		return err
	}

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

	return nil
}

func listUserRepos(userarg interface{}) error {
	var username string
	currentUser, token, _ := auth.LoadToken(false)

	if currentUser == "" {
		return fmt.Errorf("This command requires login.")
	}

	if userarg == nil {
		username = currentUser
	} else {
		username = userarg.(string)
	}

	info, err := repo.GetRepos(username, token)
	if err != nil {
		return err
	}

	fmt.Printf("Repositories owned by %s\n", username)
	for idx, r := range info {
		fmt.Printf("%d:  %s\n", idx+1, r.Name)
	}

	return nil
}

func listPubRepos() error {
	repos, err := repo.GetRepos("", "")
	fmt.Printf("Public repositories\n")
	for idx, repoInfo := range repos {
		fmt.Printf("%d: %s [head: %s]\n", idx+1, repoInfo.Name, repoInfo.Head)
		fmt.Printf(" - %s\n", repoInfo.Description)
	}
	return err
}

func main() {
	usage := `
GIN command line client

Usage:
	gin login    [<username>]
	gin create   [<name>] [-d <description>]
	gin upload   [<path>]
	gin download [<path>]
	gin repos    [<username>]
	gin info     [<username>]
	gin keys     [-v | --verbose]
	gin keys     --add <filename>
	gin public
`

	args, _ := docopt.Parse(usage, nil, true, "gin cli 0.0", false)

	var err error

	switch {
	case args["login"].(bool):
		err = auth.Login(args["<username>"])
		if err != nil {
			fmt.Println("Authentication failed!")
		}
	case args["create"].(bool):
		err = createRepo(args["<name>"], args["<description>"])
	case args["upload"].(bool):
		err = upload(args["<path>"])
	case args["download"].(bool):
		err = download(args["<path>"])
	case args["info"].(bool):
		err = printAccountInfo(args["<username>"])
	case args["keys"].(bool):
		if args["--add"].(bool) {
			err = addKey(args["<filename>"])
		} else {
			printFullKeys := false
			if args["-v"].(bool) || args["--verbose"].(bool) {
				printFullKeys = true
			}
			err = printKeys(printFullKeys)
		}
	case args["repos"].(bool):
		err = listUserRepos(args["<username>"])
	case args["public"].(bool):
		err = listPubRepos()
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
