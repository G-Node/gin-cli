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
	"github.com/docopt/docopt-go"
	"github.com/howeyc/gopass"
)

// const host = "http://localhost:8081"
const auth = "https://auth.gin.g-node.org"
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

	address := fmt.Sprintf("%s/api/accounts/%s", auth, name)
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
	url := fmt.Sprintf("%s/api/accounts?%s", auth, params.Encode())
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

func storeToken(username string, token string) error {
	userTokenStr := username + "\n" + token

	err := ioutil.WriteFile("token", []byte(userTokenStr), 0600)

	if err != nil {
		return err
	}

	return nil
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

func login(userarg interface{}) error {

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
			fmt.Println("Cancelled.")
			return err
		}
		if err == gopass.ErrMaxLengthExceeded {
			fmt.Println("Error: Input too long.")
			return err
		}
	}

	password = string(pwbytes)

	if password == "" {
		fmt.Println("No password provided. Aborting.")
		return err
	}

	params := url.Values{}
	params.Add("scope", "repo-read repo-write account-read account-write")
	params.Add("username", username)
	params.Add("password", password)
	params.Add("grant_type", "password")
	params.Add("client_id", "gin")
	params.Add("client_secret", "secret")

	address := fmt.Sprintf("%s/oauth/token", auth)

	req, _ := http.NewRequest("POST", address, strings.NewReader(params.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := http.Client{}
	res, err := client.Do(req)
	var authresp proto.TokenResponse

	if err != nil {
		return err
	} else if res.StatusCode != 200 {
		return fmt.Errorf("[Login error] %s", res.Status)
	}

	defer close(res.Body)

	b, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return err
	}
	err = json.Unmarshal(b, &authresp)
	if err != nil {
		return err
	}

	storeToken(username, authresp.AccessToken)

	fmt.Printf("[Login success] You are now logged in as %s\n", username)
	fmt.Printf("You have been granted the following permissions: %v\n", strings.Replace(authresp.Scope, " ", ", ", -1))

	return nil

}

func printKeys(printFull bool) error {
	username, token := loadToken()

	if username == "" {
		fmt.Println()
		return fmt.Errorf("You are not logged in.")
	}
	address := fmt.Sprintf("%s/api/accounts/%s/keys", auth, username)
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
	address := fmt.Sprintf("%s/api/accounts/%s/keys", auth, username)
	// TODO: Check err and req.StatusCode
	key := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDRpAtSInDz5M1a8nkY7TyEYx5MCvAdL+A2P5k5e7w5v8kizR7fMDtfG+PM33hEV54R2kFV+ga+JQw1GQjZfWOR71Yo3sGpRZMjr8cHGXLWmEvOemHYPrXs5FWm78X1XTXoCwmkhO7akyaPfKIHJUDsbxjjy0VsK6LHG/28fArct5s9+GDq7p46ifph1g3m6khIqGmdIZnkULZh7WIG10pJIx2HNpzYS3CSr4Er3Pmzwg0YZMRE25uJUGcsed9+s4RvbKuPyZewSqEtb4ACYCERcm3KnCKdpWfZMUB2v87Td6+eqG5YcxuAoJtK9fVqhZIslDroonnCvCXNd4WQBLwR alice-test1"

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

	address := fmt.Sprintf("%s/api/accounts/%s", auth, username)
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

func main() {
	usage := `
GIN command line client

Usage:
	gin login [<username>]
	gin info  [<username>]
	gin keys  [-v | --verbose]
	gin keys add
`

	args, _ := docopt.Parse(usage, nil, true, "gin cli 0.0", false)

	// akerr := addKey()
	// if akerr != nil {
	// 	fmt.Println(akerr)
	// 	os.Exit(1)
	// }

	switch {
	case args["login"].(bool):
		err := login(args["<username>"])
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
	}

}
