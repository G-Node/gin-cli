package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"

	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/G-Node/gin-core/gin"
	"github.com/howeyc/gopass"
)

const authhost = "https://auth.gin.g-node.org"

// Client is a client interface to the auth server. Embeds web.Client.
type Client struct {
	*web.Client
}

// NewClient returns a new client for the auth server.
func NewClient() *Client {
	serverURL := authhost
	return &Client{web.NewClient(serverURL)}
}

// NewKey is used for adding new public keys to gin-auth
type NewKey struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Temporary   bool   `json:"temporary"`
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

	defer web.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	// util.CheckError(err)
	if err != nil {
		return keys, err
	}
	err = json.Unmarshal(b, &keys)
	// util.CheckError(err)
	return keys, err
}

// RequestAccount requests a specific account by name
func (authcl Client) RequestAccount(name, token string) (gin.Account, error) {
	var acc gin.Account

	// authcl := client.NewClient(authhost)
	authcl.Token = token
	res, err := authcl.Get("/api/accounts/" + name)
	// util.CheckErrorMsg(err, "[Account retrieval] Request failed.")
	if err != nil {
		return acc, err
	} else if res.StatusCode != 200 {
		// util.Die(fmt.Sprintf("[Account retrieval] Failed. Server returned: %s", res.Status))
		return acc, fmt.Errorf("[Account retrieval] Failed. Server returned %s", res.Status)
	}

	defer web.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	err = json.Unmarshal(b, &acc)
	return acc, err
}

// SearchAccount Search for account
func (authcl Client) SearchAccount(query string) ([]gin.Account, error) {
	var accs []gin.Account

	params := url.Values{}
	params.Add("q", query)
	// authcl := client.NewClient(authhost)
	address := fmt.Sprintf("/api/accounts?%s", params.Encode())
	res, err := authcl.Get(address)
	// util.CheckErrorMsg(err, "[Account search] Request failed.")
	if err != nil {
		return accs, err
	} else if res.StatusCode != 200 {
		// util.Die(fmt.Sprintf("[Account search] Failed. Server returned: %s", res.Status))
		return accs, fmt.Errorf("[Account search] Failed. Server returned: %s", res.Status)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return accs, err
	}

	err = json.Unmarshal(body, &accs)
	return accs, err
}

// AddKey adds the given key to the current user's authorised keys
func AddKey(key, description string, temp bool) error {
	username, token, err := LoadToken(true)
	util.CheckErrorMsg(err, "This command requires login.")

	address := fmt.Sprintf("/api/accounts/%s/keys", authcl.Username)
	data := NewKey{Key: key, Description: description, Temporary: temp}
	res, err := authcl.Post(address, data)

	// util.CheckErrorMsg(err, "[Add key] Request failed.")
	if err != nil {
		return err
	} else if res.StatusCode != 200 {
		// util.Die(fmt.Sprintf("[Add key] Failed. Server returned %s", res.Status))
		return fmt.Errorf("[Add key] Failed. Server returned %s", res.Status)
	}
	web.CloseRes(res.Body)
	return nil
}
