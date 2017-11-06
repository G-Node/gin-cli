package ginclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"

	"net/http"

	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/G-Node/gin-core/gin"
	gogs "github.com/gogits/go-gogs-client"
)

// GINUser represents a API user.
type GINUser struct {
	ID        int64  `json:"id"`
	UserName  string `json:"login"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// Client is a client interface to the GIN server. Embeds web.Client.
type Client struct {
	*web.Client
	GitHost string
	GitUser string
}

// NewClient returns a new client for the GIN server.
func NewClient(host string) *Client {
	return &Client{Client: web.NewClient(host)}
}

// GetUserKeys fetches the public keys that the user has added to the auth server.
func (gincl *Client) GetUserKeys() ([]gogs.PublicKey, error) {
	var keys []gogs.PublicKey
	err := gincl.LoadToken()
	if err != nil {
		return keys, fmt.Errorf("This command requires login")
	}

	res, err := gincl.Get("/api/v1/user/keys")
	if err != nil {
		return keys, fmt.Errorf("Request for keys returned error")
	} else if res.StatusCode != http.StatusOK {
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
func (gincl *Client) RequestAccount(name string) (gogs.User, error) {
	var acc gogs.User
	res, err := gincl.Get(fmt.Sprintf("/api/v1/users/%s", name))
	if err != nil {
		return acc, err
	} else if res.StatusCode == http.StatusNotFound {
		return acc, fmt.Errorf("User '%s' does not exist", name)
	} else if res.StatusCode != http.StatusOK {
		return acc, fmt.Errorf("Unknown error during user lookup for '%s'\nThe server returned '%s'", name, res.Status)
	}

	defer web.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return acc, err
	}
	err = json.Unmarshal(b, &acc)
	return acc, err
}

// SearchAccount retrieves a list of accounts that match the query string.
func (gincl *Client) SearchAccount(query string) ([]gin.Account, error) {
	var accs []gin.Account

	params := url.Values{}
	params.Add("q", query)
	address := fmt.Sprintf("/api/accounts?%s", params.Encode())
	res, err := gincl.Get(address)
	if err != nil {
		return accs, err
	} else if res.StatusCode != http.StatusOK {
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
// If force is enabled, any key which matches the new key's description will be overwritten.
func (gincl *Client) AddKey(key, description string, force bool) error {
	err := gincl.LoadToken()
	if err != nil {
		return err
	}
	newkey := gogs.PublicKey{Key: key, Title: description}

	if force {
		// Attempting to delete potential existing key that matches the title
		_ = gincl.DeletePubKey(description)
	}

	address := fmt.Sprintf("/api/v1/user/keys")
	res, err := gincl.Post(address, newkey)
	if err != nil {
		return err
	} else if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("[Add key] Failed. Server returned %s", res.Status)
	}
	web.CloseRes(res.Body)
	return nil
}

// DeletePubKey removes the key that matches the given description (title) from the current user's authorised keys.
func (gincl *Client) DeletePubKey(description string) error {
	err := gincl.LoadToken()
	if err != nil {
		return err
	}

	keys, err := gincl.GetUserKeys()
	if err != nil {
		util.LogWrite("Error when getting user keys: %v", err)
	}

	for _, key := range keys {
		if key.Title == description {
			address := fmt.Sprintf("/api/v1/user/keys/%d", key.ID)
			res, err := gincl.Delete(address)
			if err != nil {
				return err
			} else if res.StatusCode != http.StatusNoContent {
				return fmt.Errorf("[Del key] Failed. Server returned %s", res.Status)
			}
			web.CloseRes(res.Body)
			// IDs are unique, so we can break after the first match
			break
		}
	}

	return nil
}

// Login requests a token from the auth server and stores the username and token to file.
// It also generates a key pair for the user for use in git commands.
func (gincl *Client) Login(username, password, clientID string) error {
	tokenCreate := &gogs.CreateAccessTokenOption{Name: "gin-cli"}
	address := fmt.Sprintf("/api/v1/users/%s/tokens", username)
	resp, err := gincl.PostBasicAuth(address, username, password, tokenCreate)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("[Login] Request failed: %s", resp.Status)
		}
		return fmt.Errorf("[Login] Request failed. No response from server")
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("[Login] Failed. Check username and password: %s", resp.Status)
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	util.LogWrite("Got response: %s", resp.Status)
	token := AccessToken{}
	err = json.Unmarshal(data, &token)
	if err != nil {
		return err
	}
	gincl.Username = username
	gincl.Token = token.Sha1
	util.LogWrite("Login successful. Username: %s", username)

	err = gincl.StoreToken()
	if err != nil {
		return fmt.Errorf("Error while storing token: %s", err.Error())
	}

	return gincl.MakeSessionKey()
}

// Logout logs out the currently logged in user in 3 steps:
// 1. Remove the public key matching the current hostname from the server.
// 2. Delete the private key file from the local machine.
// 3. Delete the user token.
func (gincl *Client) Logout() {
	// 1. Delete public key
	hostname, err := os.Hostname()
	if err != nil {
		util.LogWrite("Could not retrieve hostname")
		hostname = defaultHostname
	}

	currentkeyname := fmt.Sprintf("%s@%s", gincl.Username, hostname)
	_ = gincl.DeletePubKey(currentkeyname)

	// 2. Delete private key
	privKeyFile := util.PrivKeyPath(gincl.UserToken.Username)
	err = os.Remove(privKeyFile)
	if err != nil {
		util.LogWrite("Error deleting key file")
	} else {
		util.LogWrite("Private key file deleted")
	}

	err = web.DeleteToken()
	if err != nil {
		util.LogWrite("Error deleting token file")
	}
}

// AccessToken represents a API access token.
type AccessToken struct {
	Name string `json:"name"`
	Sha1 string `json:"sha1"`
}
