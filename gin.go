package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/G-Node/gin-cli/proto"
	"github.com/docopt/docopt-go"
	"github.com/howeyc/gopass"
)

func close(b io.ReadCloser) {
	err := b.Close()
	if err != nil {
		fmt.Println("Error during cleanup:", err)
	}
}

const host = "http://localhost:8081"

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

func login(user string, pass string) (proto.AuthResponse, error) {

	params := url.Values{}
	params.Add("scope", "repo-read repo-write account-read account-write")
	params.Add("username", user)
	params.Add("password", pass)
	params.Add("grant_type", "password")
	params.Add("client_id", "gin")
	params.Add("client_secret", "secret")

	address := fmt.Sprintf("%s/oauth/token", host)

	req, _ := http.NewRequest("POST", address, strings.NewReader(params.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	client := http.Client{}
	res, err := client.Do(req)
	defer close(res.Body)
	var authresp proto.AuthResponse

	if err != nil {
		return authresp, err
	} else if status := res.StatusCode; status != 200 {
		return authresp, fmt.Errorf("[Login error] Server returned status: %d", status)
	}

	b, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return authresp, err
	}
	err = json.Unmarshal(b, &authresp)
	if err != nil {
		return authresp, err
	}

	return authresp, nil

}

// GetSSHKeys return logged in user's SSH keys
func GetSSHKeys(user string, token string) []string {
	address := fmt.Sprintf("%s/api/accounts/%s/keys", host, user)
	req, _ := http.NewRequest("GET", address, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	client := http.Client{}
	res, err := client.Do(req)
	defer close(res.Body)

	if err != nil {
		fmt.Println("Request for keys returned error:", err)
		return nil
	} else if status := res.StatusCode; status != 200 {
		fmt.Println("Request for keys returned status code", status)
		return nil
	}

	b, err := ioutil.ReadAll(res.Body)

	var keyInfo []proto.SSHKey

	err = json.Unmarshal(b, &keyInfo)

	var keys = make([]string, len(keyInfo))

	for idx, k := range keyInfo {
		keys[idx] = k.Key
	}

	return keys
}

func main() {
	usage := `
GIN command line client

Usage:
	gin login [<username>]

`

	args, _ := docopt.Parse(usage, nil, true, "gin cli 0.0", false)

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
			return
		}
		if err == gopass.ErrMaxLengthExceeded {
			fmt.Println("Error: Input too long.")
			return
		}
	}

	password = string(pwbytes)

	if password == "" {
		fmt.Println("No password provided. Aborting.")
		return
	}

	auth, err := login(username, password)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("[Login success] You are now logged in as %s\n", username)
	fmt.Printf("The following permissions have been granted: %v\n", strings.Replace(auth.Scope, " ", ", ", -1))

	keys := GetSSHKeys(username, auth.AccessToken)
	fmt.Printf("\nKeys for user %s:\n", username)
	for _, k := range keys {
		fmt.Println("\t-", k)

	}
}
