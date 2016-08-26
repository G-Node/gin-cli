package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/G-Node/gin-auth/proto"
	"github.com/docopt/docopt-go"
	"github.com/howeyc/gopass"
)

const host = "http://localhost:8081"

func close(b io.ReadCloser) {
	err := b.Close()
	if err != nil {
		fmt.Println("Error during cleanup:", err)
	}
}

func buffAppend(buffer *bytes.Buffer, str string) {
	if str != "" {
		buffer.WriteString(fmt.Sprintf("%s ", str))
	}
}

// RequestAccount requests a specific account by name
func RequestAccount(name string) (proto.Account, error) {
	var acc proto.Account

	address := fmt.Sprintf("%s/api/accounts/%s", host, name)
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
	url := fmt.Sprintf("%s/api/accounts?%s", host, params.Encode())
	res, err := http.Get(url)

	if err != nil {
		return results, err
	} else if status := res.StatusCode; status != 200 {
		return results, fmt.Errorf("[Account search error] Server returned status: %d", status)
	}

	body, _ := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(body, &results)

	return results, nil
}

func login(args map[string]interface{}) error {

	var username, password string

	if args["<username>"] == nil {
		username = ""
	} else {
		username = args["<username>"].(string)
	}

	if username == "" {
		// prompt for login
		fmt.Print("Login: ")
		username = ""
		fmt.Scanln(&username)
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

	address := fmt.Sprintf("%s/oauth/token", host)

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

	// store token
	err = ioutil.WriteFile("token", []byte(authresp.AccessToken), 0600)

	if err != nil {
		return err
	}

	fmt.Printf("[Login success] You are now logged in as %s\n", username)
	fmt.Printf("You have been granted the following permissions: %v\n", strings.Replace(authresp.Scope, " ", ", ", -1))

	return nil

}

// GetSSHKeys return logged in user's SSH keys
func GetSSHKeys(user string, token string) []proto.SSHKey {
	address := fmt.Sprintf("%s/api/accounts/%s/keys", host, user)
	// TODO: Check err and req.StatusCode
	req, _ := http.NewRequest("GET", address, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := http.Client{}
	res, err := client.Do(req)

	if err != nil {
		fmt.Println("Request for keys returned error:", err)
		return nil
	} else if status := res.StatusCode; status != 200 {
		fmt.Println("Request for keys returned status code", status)
		return nil
	}

	defer close(res.Body)

	b, err := ioutil.ReadAll(res.Body)

	var keys []proto.SSHKey

	err = json.Unmarshal(b, &keys)

	return keys
}

func printAccountInfo(args map[string]interface{}) error {

	// assume username was specified for now
	// TODO: Resolve logged in username if no username was provided
	var username string

	if args["<username>"] == nil {
		username = ""
	} else {
		username = args["<username>"].(string)
	}

	if username == "" {
		// prompt for login
		fmt.Print("Specify username for info lookup: ")
		username = ""
		fmt.Scanln(&username)
	}

	address := fmt.Sprintf("%s/api/accounts/%s", host, username)
	req, err := http.NewRequest("GET", address, nil)
	token := "" // TODO: Get token from file
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := http.Client{}
	res, err := client.Do(req)

	if err != nil {
		fmt.Printf("[Error] Account information request failed: %s\n", err)
	} else if res.StatusCode != 200 {
		fmt.Printf("Request failed. Server returned; %s", res.Status)
	}

	defer close(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	var info proto.Account

	err = json.Unmarshal(b, &info)

	var fullnameBuffer bytes.Buffer

	buffAppend(&fullnameBuffer, info.Title)
	buffAppend(&fullnameBuffer, info.FirstName)
	buffAppend(&fullnameBuffer, info.MiddleName)
	buffAppend(&fullnameBuffer, info.LastName)

	var outBuffer bytes.Buffer

	fmt.Printf("Username: %s\nFull name: %s\nEmail: %v\nAffiliation: %v\n",
		info.Login, fullnameBuffer.String(), info.Email, info.Affiliation)

	return nil
}

func main() {
	usage := `
GIN command line client

Usage:
	gin login       [<username>]
	gin accountinfo [<username>]

`

	args, _ := docopt.Parse(usage, nil, true, "gin cli 0.0", false)

	if args["login"].(bool) {
		err := login(args)
		if err != nil {
			fmt.Println("Authentication failed!")
		}
	} else if args["accountinfo"].(bool) {
		err := printAccountInfo(args)
		if err != nil {
			fmt.Println("Error looking up account information.")
		}
	}

	// keys := GetSSHKeys()
	// fmt.Printf("\nKey fingerprints:\n")

	// for _, k := range keys {
	// 	fmt.Printf("\t• %s\n", k.Fingerprint)
	// }

}
