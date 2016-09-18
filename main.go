package main

// TODO: Nicer error handling. Print useful, descriptive messages.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/G-Node/gin-auth/proto"
	"github.com/G-Node/gin-cli/auth"
	"github.com/docopt/docopt-go"
)

// const host = "http://localhost:8081"
const authhost = "https://auth.gin.g-node.org"
const repo = "https://repo.gin.g-node.org"

func close(b io.ReadCloser) {
	err := b.Close()
	if err != nil {
		fmt.Println("Error during cleanup:", err)
	}
}

// condAppend Conditionally append to a buffer
func condAppend(b *bytes.Buffer, str *string) {
	if str != nil && *str != "" {
		b.WriteString(*str + " ")
	}
}

// RequestAccount requests a specific account by name
func RequestAccount(name string) (proto.Account, error) {
	var acc proto.Account

	address := fmt.Sprintf("%s/api/accounts/%s", authhost, name)
	res, err := http.Get(address)

	if err != nil {
		return acc, err
	}
	defer close(res.Body)

	b, err := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(b, &acc)
	if err != nil {
		return acc, err
	}
	return acc, nil
}

// SearchAccount Search for account
func SearchAccount(query string) ([]proto.Account, error) {
	var results []proto.Account

	params := url.Values{}
	params.Add("q", query)
	url := fmt.Sprintf("%s/api/accounts?%s", authhost, params.Encode())
	res, err := http.Get(url)

	if err != nil {
		return results, err
	} else if res.StatusCode != 200 {
		return results, fmt.Errorf("[Account search error] Server returned: %s", res.Status)
	}

	body, _ := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(body, &results)

	return results, nil
}

func loadToken() (string, string) {
	userTokenBytes, err := ioutil.ReadFile("token")
	var username, token string

	if err == nil {
		userTokenString := string(userTokenBytes)
		userToken := strings.Split(userTokenString, "\n")
		username = userToken[0]
		token = userToken[1]
	}
	// TODO: Handle error

	return username, token
}

func printKeys(printFull bool) error {
	username, token := loadToken()

	if username == "" {
		fmt.Println()
		return fmt.Errorf("You are not logged in.")
	}
	address := fmt.Sprintf("%s/api/accounts/%s/keys", authhost, username)
	// TODO: Check err and req.StatusCode
	req, _ := http.NewRequest("GET", address, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := http.Client{}
	res, err := client.Do(req)

	if err != nil {
		return fmt.Errorf("Request for keys returned error: %s", err)
	} else if res.StatusCode != 200 {
		return fmt.Errorf("[Keys request error] Server returned: %s", res.Status)
	}

	defer close(res.Body)

	b, err := ioutil.ReadAll(res.Body)

	var keys []proto.SSHKey
	err = json.Unmarshal(b, &keys)

	nkeys := len(keys)

	var message string
	if nkeys == 0 {
		message = "There are no keys "
	} else if nkeys == 1 {
		message = "You have 1 key"
	} else {
		message = fmt.Sprintf("%v keys are", nkeys)
	}
	fmt.Printf("%s associated with your account.\n\n", message)
	for idx, key := range keys {
		fmt.Printf("  [%v] \"%s\"\n", idx+1, key.Description)
		fmt.Printf("  Fingerprint: %s\n", key.Fingerprint)
		if printFull {
			fmt.Printf("\n%s\n", key.Key)
		}
	}

	return err
}

func addKey() error {

	// TODO: Prompt user for key information
	// TODO: Allow use to speciry pubkey file (default to ~/.ssh/id_rsa.pub ?)
	username, token := loadToken()

	if username == "" {
		fmt.Println()
		return fmt.Errorf("You are not logged in.")
	}
	address := fmt.Sprintf("%s/api/accounts/%s/keys", authhost, username)
	// TODO: Check err and req.StatusCode
	key := ""

	mkBody := func(key, description string) io.Reader {
		pw := &struct {
			Key         string `json:"key"`
			Description string `json:"description"`
		}{key, description}
		b, _ := json.Marshal(pw)
		return bytes.NewReader(b)
	}

	req, _ := http.NewRequest("POST", address, mkBody(key, "ll"))
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := http.Client{}
	res, err := client.Do(req)

	if err != nil {
		return fmt.Errorf("Error: %s", err)
	} else if res.StatusCode != 200 {
		return fmt.Errorf("[Add key error] Server returned: %s", res.Status)
	}

	close(res.Body)
	return nil
}

func printAccountInfo(userarg interface{}) error {
	var username string
	currentUser, token := loadToken()

	if userarg == nil {
		username = currentUser
	} else {
		username = userarg.(string)
	}

	if username == "" {
		// prompt for login
		fmt.Print("Specify username for info lookup: ")
		username = ""
		fmt.Scanln(&username)
	}

	address := fmt.Sprintf("%s/api/accounts/%s", authhost, username)
	req, err := http.NewRequest("GET", address, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := http.Client{}
	res, err := client.Do(req)

	if err != nil {
		fmt.Printf("[Error] Request failed: %s\n", err)
		return err
	} else if res.StatusCode != 200 {
		return fmt.Errorf("[Account search error] Server returned: %s", res.Status)
	}

	defer close(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	var info proto.Account

	err = json.Unmarshal(b, &info)

	var fullnameBuffer bytes.Buffer

	condAppend(&fullnameBuffer, info.Title)
	condAppend(&fullnameBuffer, &info.FirstName)
	condAppend(&fullnameBuffer, info.MiddleName)
	condAppend(&fullnameBuffer, &info.LastName)

	var outBuffer bytes.Buffer

	outBuffer.WriteString(fmt.Sprintf("User [%s]\nName: %s\n", info.Login, fullnameBuffer.String()))

	if info.Email != nil && info.Email.Email != "" {
		outBuffer.WriteString(fmt.Sprintf("Email: %s\n", info.Email.Email))
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
			outBuffer.WriteString(fmt.Sprintf("Affiliation: %s\n", affiliationBuffer.String()))
		}
	}

	fmt.Println(outBuffer.String())

	return nil
}

func listRepos() error {

	_, token := loadToken()

	address := fmt.Sprintf("%s/repos/public", repo)
	// TODO: Check err and req.StatusCode
	req, _ := http.NewRequest("GET", address, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := http.Client{}
	res, err := client.Do(req)

	if err != nil {
		return fmt.Errorf("Request for repos returned error: %s", err)
	} else if res.StatusCode != 200 {
		return fmt.Errorf("[Repo listing error] Server returned: %s", res.Status)
	}

	defer close(res.Body)

	b, err := ioutil.ReadAll(res.Body)

	print(string(b))

	return err
}

func main() {
	usage := `
GIN command line client

Usage:
	gin login [<username>]
	gin info  [<username>]
	gin keys  [-v | --verbose]
	gin keys add
	gin repos [<username>]
`

	args, _ := docopt.Parse(usage, nil, true, "gin cli 0.0", false)

	switch {
	case args["login"].(bool):
		err := auth.Login(args["<username>"])
		if err != nil {
			fmt.Println("Authentication failed!")
		}
	case args["info"].(bool):
		err := printAccountInfo(args["<username>"])
		if err != nil {
			fmt.Println(err)
		}
	case args["keys"].(bool):
		printFullKeys := false
		if args["-v"].(bool) || args["--verbose"].(bool) {
			printFullKeys = true
		}
		err := printKeys(printFullKeys)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	case args["repos"].(bool):
		err := listRepos()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

}
