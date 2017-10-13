package ginclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"net/http"

	"strings"

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
func (gincl *Client) AddKey(key, description string, temp bool) error {
	err := gincl.LoadToken()
	if err != nil {
		return err
	}
	newkey := gogs.PublicKey{Key: key, Title: description}
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

// DeleteKey removes the given key from the current user's authorised keys.
func (gincl *Client) DeleteKey(key gogs.PublicKey) error {
	err := gincl.LoadToken()
	if err != nil {
		return err
	}
	address := fmt.Sprintf("/api/v1/user/keys/%d", key.ID)
	res, err := gincl.Delete(address)
	if err != nil {
		return err
	} else if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("[Add key] Failed. Server returned %s", res.Status)
	}
	web.CloseRes(res.Body)
	return nil
}

func (gincl *Client) DeleteTmpKeys() error {
	keys, err := gincl.GetUserKeys()
	if err != nil {
		util.LogWrite("Error when getting user keys: %v", err)
		return err
	}
	for _, key := range keys {
		util.LogWrite("key: %s", key.Title)
		if strings.Contains(key.Title, "tmpkey") {
			// is logged
			gincl.DeleteKey(key)
		}
	}
	return err
}

// Login requests a token from the auth server and stores the username and token to file.
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
	util.LogWrite("Got response: %s,%s", string(data), string(resp.StatusCode))
	token := AccessToken{}
	err = json.Unmarshal(data, &token)
	if err != nil {
		return err
	}
	gincl.Username = username
	gincl.Token = token.Sha1
	util.LogWrite("Login successful. Username: %s, %v", username, token)

	return gincl.StoreToken()
}

// AccessToken represents a API access token.
type AccessToken struct {
	Name string `json:"name"`
	Sha1 string `json:"sha1"`
}
