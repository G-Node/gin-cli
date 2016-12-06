package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"

	"github.com/G-Node/gin-cli/client"
	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-core/gin"
	"github.com/howeyc/gopass"
)

const authhost = "https://auth.gin.g-node.org"

// Client is a client interface to the auth server. Embeds client.Client.
type Client struct {
	*client.Client
}

// NewClient returns a new client for the auth server.
func NewClient() *Client {
	serverURL := authhost
	return &Client{client.NewClient(serverURL)}
}

// NewKey is used for adding new public keys to gin-auth
type NewKey struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Temporary   bool   `json:"temporary"`
}

// LoadToken loads the auth token from the token file, checks it against the auth server,
// and sets the token and username in the auth struct.
func (authcl *Client) LoadToken() error {
	tokenfile := filepath.Join(util.ConfigPath(), "token")
	tokenBytes, err := ioutil.ReadFile(tokenfile)
	tokenInfo := gin.TokenInfo{}

	if err != nil {
		return err
	}

	token := string(tokenBytes)
	// authcl := client.NewClient(authhost)
	res, err := authcl.Get("/oauth/validate/" + token)
	if err != nil {
		// fmt.Fprintln(os.Stderr, "[Auth error] Error communicating with server.")
		return err
	}

	defer client.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, &tokenInfo)

	username := tokenInfo.Login
	if username == "" {
		return fmt.Errorf("[Auth error] You are not logged in")
	}
	authcl.Username = username
	authcl.Token = token
	return nil
}

// GetUserKeys Load token and request an slice of the user's keys
func (authcl *Client) GetUserKeys() ([]gin.SSHKey, error) {
	var keys []gin.SSHKey
	err := authcl.LoadToken()
	// util.CheckErrorMsg(err, "This command requires login.")
	if err != nil {
		return keys, fmt.Errorf("This command requires login")
	}

	// authcl := client.NewClient(authhost)
	// authcl.Token = token

	res, err := authcl.Get(fmt.Sprintf("/api/accounts/%s/keys", authcl.Username))
	// util.CheckErrorMsg(err, "Request for keys returned error.")
	if err != nil {
		return keys, fmt.Errorf("Request for keys returned error")
	} else if res.StatusCode != 200 {
		// util.Die(fmt.Sprintf("[Keys request error] Server returned: %s", res.Status))
		return keys, fmt.Errorf("[Keys request error] Server returned: %s", res.Status)
	}

	defer client.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	util.CheckError(err)
	err = json.Unmarshal(b, &keys)
	util.CheckError(err)
	return keys, nil
}

// Login Request credentials, perform login, and store token
func (authcl *Client) Login(userarg interface{}) error {

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

	res, err := authcl.Post("/oauth/token", params)
	util.CheckError(err)
	// req, _ := http.NewRequest("POST", "/oauth/token", strings.NewReader(params.Encode()))
	// req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	// res, err := client.web.Do(req)

	// if err != nil {
	// 	return nil, err
	// } else if res.StatusCode != 200 {
	// 	return nil, fmt.Errorf("[Login error] %s", res.Status)
	// }

	defer client.CloseRes(res.Body)
	// authcl := client.NewClient(authhost)
	// b, err := authcl.DoLogin(username, password)

	b, err := ioutil.ReadAll(res.Body)
	util.CheckError(err)

	var authresp gin.TokenResponse
	err = json.Unmarshal(b, &authresp)
	util.CheckError(err)

	tokenfile := filepath.Join(util.ConfigPath(), "token")
	err = ioutil.WriteFile(tokenfile, []byte(authresp.AccessToken), 0600)
	// util.CheckErrorMsg(err, "[Error] Login failed while storing token.")
	if err != nil {
		return err
	}
	fmt.Printf("[Login success] You are now logged in as %s\n", username)
	// fmt.Printf("You have been granted the following permissions: %v\n", strings.Replace(authresp.Scope, " ", ", ", -1))
	return nil
}

// RequestAccount requests a specific account by name
func (authcl Client) RequestAccount(name, token string) gin.Account {
	var acc gin.Account

	// authcl := client.NewClient(authhost)
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
func (authcl Client) SearchAccount(query string) []gin.Account {
	var results []gin.Account

	params := url.Values{}
	params.Add("q", query)
	// authcl := client.NewClient(authhost)
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

// AddKey adds the given key to the current user's authorised keys
func AddKey(key, description string, temp bool) error {
	username, token, err := LoadToken(true)
	util.CheckErrorMsg(err, "This command requires login.")

	address := fmt.Sprintf("/api/accounts/%s/keys", authcl.Username)
	data := NewKey{Key: key, Description: description, Temporary: temp}
	res, err := authcl.Post(address, data)

	util.CheckErrorMsg(err, "[Add key] Request failed.")
	if res.StatusCode != 200 {
		util.Die(fmt.Sprintf("[Add key] Failed. Server returned %s", res.Status))
	}
	client.CloseRes(res.Body)
	return nil
}
