package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/G-Node/gin-cli/client"
	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-core/gin"
	"github.com/howeyc/gopass"
)

const authhost = "https://auth.gin.g-node.org"

func storeToken(token string) error {
	tokenfile := filepath.Join(util.ConfigPath(), "token")
	return ioutil.WriteFile(tokenfile, []byte(token), 0600)
}

func noTokenWarning(warn bool) {
	if warn {
		fmt.Println("You are not logged in.")
	}
}

// LoadToken Get the current signed in username and auth token
func LoadToken(warn bool) (string, string, error) {
	tokenfile := filepath.Join(util.ConfigPath(), "token")
	tokenBytes, err := ioutil.ReadFile(tokenfile)
	tokenInfo := gin.TokenInfo{}
	var username, token string

	if err != nil {
		noTokenWarning(warn)
		return "", "", err
	}

	token = string(tokenBytes)
	authcl := client.NewClient(authhost)
	res, err := authcl.Get("/oauth/validate/" + token)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[Auth error] Error communicating with server.")
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
func GetUserKeys() []gin.SSHKey {
	username, token, err := LoadToken(true)
	util.CheckErrorMsg(err, "This command requires login.")

	authcl := client.NewClient(authhost)
	authcl.Token = token

	res, err := authcl.Get(fmt.Sprintf("/api/accounts/%s/keys", username))
	util.CheckErrorMsg(err, "Request for keys returned error.")
	if res.StatusCode != 200 {
		util.Die(fmt.Sprintf("[Keys request error] Server returned: %s", res.Status))
	}

	defer client.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	util.CheckError(err)
	var keys []gin.SSHKey
	err = json.Unmarshal(b, &keys)
	util.CheckError(err)
	return keys
}

// Login Request credentials, perform login, and store token
func Login(userarg interface{}) {

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

	authcl := client.NewClient(authhost)
	b, err := authcl.DoLogin(username, password)
	util.CheckError(err)

	var authresp gin.TokenResponse
	err = json.Unmarshal(b, &authresp)
	util.CheckError(err)

	err = storeToken(authresp.AccessToken)
	util.CheckErrorMsg(err, "[Error] Login failed while storing token.")
	fmt.Printf("[Login success] You are now logged in as %s\n", username)
	// fmt.Printf("You have been granted the following permissions: %v\n", strings.Replace(authresp.Scope, " ", ", ", -1))
}

// RequestAccount requests a specific account by name
func RequestAccount(name, token string) gin.Account {
	var acc gin.Account

	authcl := client.NewClient(authhost)
	authcl.Token = token
	res, err := authcl.Get("/api/accounts/" + name)
	util.CheckErrorMsg(err, "[Account retrieval] Request failed.")

	if res.StatusCode != 200 {
		util.Die(fmt.Sprintf("[Account retrieval] Failed. Server returned: %s", res.Status))
	}

	defer client.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	err = json.Unmarshal(b, &acc)
	util.CheckError(err)
	return acc
}

// SearchAccount Search for account
func SearchAccount(query string) []gin.Account {
	var results []gin.Account

	params := url.Values{}
	params.Add("q", query)
	authcl := client.NewClient(authhost)
	address := fmt.Sprintf("/api/accounts?%s", params.Encode())
	res, err := authcl.Get(address)
	util.CheckErrorMsg(err, "[Account search] Request failed.")

	if res.StatusCode != 200 {
		util.Die(fmt.Sprintf("[Account search] Failed. Server returned: %s", res.Status))
	}

	body, _ := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(body, &results)
	util.CheckError(err)
	return results
}

// AddKey Adds the given key to the current user's authorised keys
func AddKey(key, description string) error {
	username, token, err := LoadToken(true)
	address := fmt.Sprintf("%s/api/accounts/%s/keys", authhost, username)
	// TODO: Check err and req.StatusCode
	// TODO: clean up key struct
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
