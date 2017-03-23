package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/G-Node/gin-core/gin"
)

// Client is a client interface to the auth server. Embeds web.Client.
type Client struct {
	*web.Client
}

// NewClient returns a new client for the auth server.
func NewClient(host string) *Client {
	return &Client{web.NewClient(host)}
}

// NewKey is used for adding new public keys to gin-auth
type NewKey struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Temporary   bool   `json:"temporary"`
}

// GetUserKeys fetches the public keys that the user has added to the auth server.
func (authcl *Client) GetUserKeys() ([]gin.SSHKey, error) {
	var keys []gin.SSHKey
	err := authcl.LoadToken()
	if err != nil {
		return keys, fmt.Errorf("This command requires login")
	}

	res, err := authcl.Get(fmt.Sprintf("/api/accounts/%s/keys", authcl.Username))
	if err != nil {
		return keys, fmt.Errorf("Request for keys returned error")
	} else if res.StatusCode != 200 {
		return keys, fmt.Errorf("[Keys request error] Server returned: %s", res.Status)
	}

	defer web.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return keys, err
	}
	err = json.Unmarshal(b, &keys)
	return keys, err
}

// RequestAccount requests a specific account by name.
func (authcl *Client) RequestAccount(name string) (gin.Account, error) {
	var acc gin.Account

	res, err := authcl.Get(fmt.Sprintf("/api/accounts/%s", name))
	if err != nil {
		return acc, err
	} else if res.StatusCode != 200 {
		return acc, fmt.Errorf("User '%s' does not exist", name)
	}

	defer web.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	err = json.Unmarshal(b, &acc)
	return acc, err
}

// SearchAccount retrieves a list of accounts that match the query string.
func (authcl *Client) SearchAccount(query string) ([]gin.Account, error) {
	var accs []gin.Account

	params := url.Values{}
	params.Add("q", query)
	address := fmt.Sprintf("/api/accounts?%s", params.Encode())
	res, err := authcl.Get(address)
	if err != nil {
		return accs, err
	} else if res.StatusCode != 200 {
		return accs, fmt.Errorf("[Account search] Failed. Server returned: %s", res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return accs, err
	}

	err = json.Unmarshal(body, &accs)
	return accs, err
}

// AddKey adds the given key to the current user's authorised keys.
func (authcl *Client) AddKey(key, description string, temp bool) error {
	err := authcl.LoadToken()
	if err != nil {
		return err
	}

	address := fmt.Sprintf("/api/accounts/%s/keys", authcl.Username)
	data := NewKey{Key: key, Description: description, Temporary: temp}
	res, err := authcl.Post(address, data)

	if err != nil {
		return err
	} else if res.StatusCode != 200 {
		return fmt.Errorf("[Add key] Failed. Server returned %s", res.Status)
	}
	web.CloseRes(res.Body)
	return nil
}

// Login requests a token from the auth server and stores the username and token to file.
func (authcl *Client) Login(username, password, clientID, clientSecret string) error {
	// The struct below will be used when we switch token request to using json post data on auth
	// See https://github.com/G-Node/gin-auth/issues/112
	// params := gin.LoginRequest{
	// 	Scope:        "repo-read repo-write account-read account-write",
	// 	Username:     username,
	// 	Password:     password,
	// 	GrantType:    "password",
	// 	ClientID:     clientID,
	// 	ClientSecret: clientSecret,
	// }
	params := url.Values{}
	params.Add("scope", "repo-read repo-write account-read account-write")
	params.Add("username", username)
	params.Add("password", password)
	params.Add("grant_type", "password")
	params.Add("client_id", clientID)
	params.Add("client_secret", clientSecret)

	res, err := authcl.PostForm("/oauth/token", params)
	if err != nil {
		return err
	} else if res.StatusCode != 200 {
		return fmt.Errorf("[Login] Failed. Server returned %s", res.Status)
	}

	defer web.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	var authresp gin.TokenResponse
	err = json.Unmarshal(b, &authresp)
	if err != nil {
		return err
	}

	authcl.Username = username
	authcl.Token = authresp.AccessToken
	util.LogWrite("Login successful. Username: %s", username)
	util.LogWrite("Permissions granted: %s", strings.Replace(authresp.Scope, " ", ", ", -1))

	return authcl.StoreToken()
}
