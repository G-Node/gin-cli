package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/G-Node/gin-cli/client"
	"github.com/G-Node/gin-core/gin"
	"github.com/howeyc/gopass"
)

const authhost = "https://auth.gin.g-node.org"

func storeToken(token string) error {
	// TODO: Store token in config directory
	return ioutil.WriteFile("token", []byte(token), 0600)
}

func noTokenWarning(warn bool) {
	if warn {
		fmt.Println("You are not logged in.")
	}
}

// LoadToken Get the current signed in username and auth token
func LoadToken(warn bool) (string, string, error) {
	// TODO: Load token from config directory
	tokenBytes, err := ioutil.ReadFile("token")
	tokenInfo := gin.TokenInfo{}
	var username, token string

	if err != nil {
		noTokenWarning(warn)
		return "", "", nil
	}

	token = string(tokenBytes)
	authcl := client.NewClient(authhost)
	res, err := authcl.Get("/oauth/validate/" + token)
	if err != nil {
		fmt.Println("[Auth error] Error communicating with server.")
		return "", "", err
	}

	defer client.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", "", err
	}

	err = json.Unmarshal(b, &tokenInfo)

	username = tokenInfo.Login
	if username == "" && warn {
		noTokenWarning(warn)
		token = ""
	}
	return username, token, nil
}

// GetUserKeys Load token and request an slice of the user's keys
func GetUserKeys() ([]gin.SSHKey, error) {
	username, token, err := LoadToken(true)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("This command requires login.")
	}

	authcl := client.NewClient(authhost)
	authcl.Token = token

	res, err := authcl.Get(fmt.Sprintf("/api/accounts/%s/keys", username))

	if err != nil {
		return nil, fmt.Errorf("Request for keys returned error: %s", err)
	} else if res.StatusCode != 200 {
		return nil, fmt.Errorf("[Keys request error] Server returned: %s", res.Status)
	}

	defer client.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	var keys []gin.SSHKey
	err = json.Unmarshal(b, &keys)

	return keys, nil
}

// Login Request credentials, perform login, and store token
func Login(userarg interface{}) error {

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
			return nil
		}
		if err == gopass.ErrMaxLengthExceeded {
			fmt.Println("Error: Input too long.")
			return err
		}
		return err
	}

	password = string(pwbytes)

	if password == "" {
		fmt.Println("No password provided. Aborting.")
		return err
	}

	authcl := client.NewClient(authhost)
	b, err := authcl.DoLogin(username, password)
	var authresp gin.TokenResponse
	err = json.Unmarshal(b, &authresp)

	if err != nil {
		return err
	}

	err = storeToken(authresp.AccessToken)

	if err != nil {
		// Login success but unable to store token in file. Print error.
		return err
	}

	fmt.Printf("[Login success] You are now logged in as %s\n", username)
	fmt.Printf("You have been granted the following permissions: %v\n", strings.Replace(authresp.Scope, " ", ", ", -1))

	return nil

}

// RequestAccount requests a specific account by name
func RequestAccount(name, token string) (gin.Account, error) {
	var acc gin.Account

	authcl := client.NewClient(authhost)
	authcl.Token = token
	res, err := authcl.Get("/api/accounts/" + name)

	if err != nil {
		fmt.Printf("[Error] Request failed: %s\n", err)
		return acc, err
	} else if res.StatusCode != 200 {
		return acc, fmt.Errorf("[Account search error] Server returned: %s", res.Status)
	}

	defer client.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(b, &acc)
	if err != nil {
		return acc, err
	}
	return acc, nil
}

// SearchAccount Search for account
func SearchAccount(query string) ([]gin.Account, error) {
	var results []gin.Account

	params := url.Values{}
	params.Add("q", query)
	url := fmt.Sprintf("%s/api/accounts?%s", authhost, params.Encode())
	authcl := client.NewClient(authhost)
	res, err := authcl.Get(url)

	if err != nil {
		return results, err
	} else if res.StatusCode != 200 {
		return results, fmt.Errorf("[Account search error] Server returned: %s", res.Status)
	}

	body, _ := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(body, &results)

	return results, nil
}

// AddKey Adds the given key to the current user's authorised keys
func AddKey(key, description string) error {

	username, token, err := LoadToken(true)

	if err != nil {
		return nil
	}

	address := fmt.Sprintf("%s/api/accounts/%s/keys", authhost, username)
	// TODO: Check err and req.StatusCode

	mkBody := func(key, description string) io.Reader {
		pw := &struct {
			Key         string `json:"key"`
			Description string `json:"description"`
		}{key, description}
		b, _ := json.Marshal(pw)
		return bytes.NewReader(b)
	}

	req, _ := http.NewRequest("POST", address, mkBody(key, description))
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	authcl := http.Client{}
	res, err := authcl.Do(req)

	if err != nil {
		return fmt.Errorf("Error: %s", err)
	} else if res.StatusCode != 200 {
		return fmt.Errorf("[Add key error] Server returned: %s", res.Status)
	}

	client.CloseRes(res.Body)
	return nil

}
