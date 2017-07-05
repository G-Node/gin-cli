package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"

	"net/http"
	"path"

	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-cli/web"
	"github.com/G-Node/gin-core/gin"
	gogs "github.com/gogits/go-gogs-client"
)

// PublicKey is used to represent the information of the public part of a key pair.
type PublicKey struct {
	ID    int64  `json:"id"`
	Key   string `json:"key"`
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
}

// GINUser represents a API user.
type GINUser struct {
	ID        int64  `json:"id"`
	UserName  string `json:"login"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

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

// GogsPublicKey is used to represent the information of the public part of a key pair (Gogs API only).
type GogsPublicKey struct {
	Key   string `json:"key"`
	Title string `json:"title,omitempty"`
}

// GetUserKeys fetches the public keys that the user has added to the auth server.
func (authcl *Client) GetUserKeys() ([]gin.SSHKey, error) {
	gogKeys := make([]*PublicKey, 0, 10)
	var keys []gin.SSHKey
	err := authcl.LoadToken()
	if err != nil {
		return keys, fmt.Errorf("This command requires login")
	}

	res, err := authcl.Get(fmt.Sprintf("/api/v1/user/keys"))
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
	err = json.Unmarshal(b, &gogKeys)
	for _, element := range gogKeys {
		keys = append(keys, gin.SSHKey{Description: element.Title, Key: element.Key})
	}
	return keys, err
}

// RequestAccount requests a specific account by name.
func (authcl *Client) RequestAccount(name string) (gin.Account, error) {
	var acc gin.Account
	gogsUser := GINUser{}
	res, err := authcl.Get(fmt.Sprintf("/api/v1/users/%s", name))
	if err != nil {
		return acc, err
	} else if res.StatusCode == 404 {
		return acc, fmt.Errorf("User '%s' does not exist", name)
	} else if res.StatusCode != 200 {
		return acc, fmt.Errorf("Unknown error during user lookup for '%s'\nThe server returned '%s'", name, res.Status)
	}

	defer web.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	if err := json.Unmarshal(b, &gogsUser); err != nil {
		return gin.Account{}, err
	}
	acc.LastName = gogsUser.FullName
	acc.Login = gogsUser.UserName
	acc.Email = &gin.Email{Email: gogsUser.Email}

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
	gogsKey := GogsPublicKey{Key: key, Title: description}
	address := fmt.Sprintf("/api/v1/user/keys")
	res, err := authcl.Post(address, gogsKey)
	if err != nil {
		return err
	} else if res.StatusCode != http.StatusCreated {
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
	resp, err := authcl.GLogin(username, password)

	if err != nil {
		return err
	}
	data, _ := ioutil.ReadAll(resp.Body)
	util.LogWrite("Got response: %s,%s", string(data), string(resp.StatusCode))
	token := AccessToken{}
	json.Unmarshal(data, &token)
	authcl.Username = username
	authcl.Token = token.Sha1
	util.LogWrite("Login successful. Username: %s, %v", username, token)

	return authcl.StoreToken()
}

func BasicAuthEncode(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}

// AccessToken represents a API access token.
type AccessToken struct {
	Name string `json:"name"`
	Sha1 string `json:"sha1"`
}

func urlJoin(parts ...string) string {
	// First part must be a valid URL
	u, err := url.Parse(parts[0])
	util.CheckErrorMsg(err, "Bad URL in urlJoin")

	for _, part := range parts[1:] {
		u.Path = path.Join(u.Path, part)
	}
	return u.String()
}
