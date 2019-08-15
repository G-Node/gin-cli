package ginclient

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"net/http"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/G-Node/gin-cli/git"
	"github.com/G-Node/gin-cli/git/shell"
	"github.com/G-Node/gin-cli/web"
	gogs "github.com/gogits/go-gogs-client"
)

// High level functions for managing user auth.
// These functions end up performing web calls (using the web package).

// ginerror convenience alias to util.Error
type ginerror = shell.Error

// Client is a client interface to the GIN server.  Uses web.Client.
type Client struct {
	web      *web.Client
	git      *git.Client
	Username string
	Token    string
	srvalias string
}

func closeFile(f *os.File) {
	_ = f.Close()
}

// LoadToken reads the username and auth token from the token file for the
// configured server alias and sets the values for the Client.
func (cl *Client) LoadToken() error {
	srvalias := cl.srvalias
	if cl.Username != "" && cl.Token != "" {
		return nil
	}
	path, _ := config.Path(false) // Error can only occur when create=True
	filename := fmt.Sprintf("%s.token", srvalias)
	filepath := filepath.Join(path, filename)
	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to load user token: %s", err.Error())
	}
	defer closeFile(file)

	decoder := gob.NewDecoder(file)
	ut := &struct {
		Username string
		Token    string
	}{}
	err = decoder.Decode(ut)
	if err != nil {
		return fmt.Errorf("failed to parse user token: %s", err.Error())
	}

	cl.Username = ut.Username
	cl.Token = ut.Token
	cl.web.SetToken(cl.Token)
	return nil
}

// StoreToken saves the username and auth token to the token file.
func (cl *Client) StoreToken(srvalias string) error {
	path, err := config.Path(true)
	if err != nil {
		return err
	}
	filename := fmt.Sprintf("%s.token", srvalias)
	filepath := filepath.Join(path, filename)
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create token file %q: %s", filepath, err.Error())
	}
	defer closeFile(file)

	ut := struct {
		Username string
		Token    string
	}{
		cl.Username,
		cl.Token,
	}

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(ut)
	if err != nil {
		return fmt.Errorf("failed to store token at %q: %s", filepath, err.Error())
	}
	return nil
}

// DeleteToken deletes the token file if it exists (for finalising a logout).
func DeleteToken(srvalias string) error {
	path, _ := config.Path(false) // Error can only occur when create=True
	filename := fmt.Sprintf("%s.token", srvalias)
	tokenpath := filepath.Join(path, filename)
	err := os.Remove(tokenpath)
	if err != nil {
		return fmt.Errorf("could not delete token at %q: %s", tokenpath, err.Error())
	}
	return nil
}

// GINUser represents a API user.
type GINUser struct {
	ID        int64  `json:"id"`
	UserName  string `json:"login"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// New returns a new client for the GIN server, configured with the server
// referred to by the alias in the argument.  If alias is an empty or invalid
// name, an empty Client is returned.  This Client can be used for offline
// operations.
func New(alias string) *Client {
	if alias == "" {
		return &Client{web: web.New(""), srvalias: ""}
	}
	srvcfg, ok := config.Read().Servers[alias]
	if !ok {
		return &Client{web: web.New(""), srvalias: ""}
	}
	return &Client{web: web.New(srvcfg.Web.AddressStr()), srvalias: alias}
}

// AccessToken represents a API access token.
type AccessToken struct {
	Name string `json:"name"`
	Sha1 string `json:"sha1"`
}

// GitAddress returns the full address string for the configured git server
func (gincl *Client) GitAddress() string {
	if gincl.srvalias == "" {
		return ""
	}
	return config.Read().Servers[gincl.srvalias].Git.AddressStr()
}

// WebAddress returns the full address string for the configured web server
func (gincl *Client) WebAddress() string {
	return config.Read().Servers[gincl.srvalias].Web.AddressStr()
}

// GetUserKeys fetches the public keys that the user has added to the auth server.
func (gincl *Client) GetUserKeys() ([]gogs.PublicKey, error) {
	fn := "GetUserKeys()"
	var keys []gogs.PublicKey
	res, err := gincl.web.Get("/api/v1/user/keys")
	if err != nil {
		return nil, err // return error from Get() directly
	}
	switch code := res.StatusCode; {
	case code == http.StatusUnauthorized:
		return nil, ginerror{UError: res.Status, Origin: fn, Description: "authorisation failed"}
	case code == http.StatusInternalServerError:
		return nil, ginerror{UError: res.Status, Origin: fn, Description: "server error"}
	case code != http.StatusOK:
		return nil, ginerror{UError: res.Status, Origin: fn} // Unexpected error
	}

	defer web.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, ginerror{UError: err.Error(), Origin: fn, Description: "failed to read response body"}
	}
	err = json.Unmarshal(b, &keys)
	if err != nil {
		return nil, ginerror{UError: err.Error(), Origin: fn, Description: "failed to parse response body"}
	}
	return keys, nil
}

// RequestAccount requests a specific account by name.
func (gincl *Client) RequestAccount(name string) (gogs.User, error) {
	fn := fmt.Sprintf("RequestAccount(%s)", name)
	var acc gogs.User
	res, err := gincl.web.Get(fmt.Sprintf("/api/v1/users/%s", name))
	if err != nil {
		return acc, err // return error from Get() directly
	}
	switch code := res.StatusCode; {
	case code == http.StatusNotFound:
		return acc, ginerror{UError: res.Status, Origin: fn, Description: fmt.Sprintf("requested user '%s' does not exist", name)}
	case code == http.StatusUnauthorized:
		return acc, ginerror{UError: res.Status, Origin: fn, Description: "authorisation failed"}
	case code == http.StatusInternalServerError:
		return acc, ginerror{UError: res.Status, Origin: fn, Description: "server error"}
	case code != http.StatusOK:
		return acc, ginerror{UError: res.Status, Origin: fn} // Unexpected error
	}

	defer web.CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return acc, ginerror{UError: err.Error(), Origin: fn, Description: "failed to read response body"}
	}
	err = json.Unmarshal(b, &acc)
	if err != nil {
		err = ginerror{UError: err.Error(), Origin: fn, Description: "failed to parse response body"}
	}
	return acc, err
}

// AddKey adds the given key to the current user's authorised keys.
// If force is enabled, any key which matches the new key's description will be overwritten.
func (gincl *Client) AddKey(key, description string, force bool) error {
	fn := "AddKey()"
	newkey := gogs.PublicKey{Key: key, Title: description}

	if force {
		// Attempting to delete potential existing key that matches the title
		_ = gincl.DeletePubKeyByTitle(description)
	}
	res, err := gincl.web.Post("/api/v1/user/keys", newkey)
	if err != nil {
		return err // return error from Post() directly
	}
	switch code := res.StatusCode; {
	case code == http.StatusUnprocessableEntity:
		return ginerror{UError: res.Status, Origin: fn, Description: "invalid key or key with same name already exists"}
	case code == http.StatusUnauthorized:
		return ginerror{UError: res.Status, Origin: fn, Description: "authorisation failed"}
	case code == http.StatusInternalServerError:
		return ginerror{UError: res.Status, Origin: fn, Description: "server error"}
	case code != http.StatusCreated:
		return ginerror{UError: res.Status, Origin: fn} // Unexpected error
	}
	web.CloseRes(res.Body)
	return nil
}

// DeletePubKey the key with the given ID from the current user's authorised keys.
func (gincl *Client) DeletePubKey(id int64) error {
	fn := "DeletePubKey()"

	address := fmt.Sprintf("/api/v1/user/keys/%d", id)
	res, err := gincl.web.Delete(address)
	defer web.CloseRes(res.Body)
	if err != nil {
		return err // Return error from Delete() directly
	}
	switch code := res.StatusCode; {
	case code == http.StatusInternalServerError:
		return ginerror{UError: res.Status, Origin: fn, Description: "server error"}
	case code == http.StatusUnauthorized:
		return ginerror{UError: res.Status, Origin: fn, Description: "authorisation failed"}
	case code == http.StatusForbidden:
		return ginerror{UError: res.Status, Origin: fn, Description: "failed to delete key (forbidden)"}
	case code != http.StatusNoContent:
		return ginerror{UError: res.Status, Origin: fn} // Unexpected error
	}
	return nil
}

// DeletePubKeyByTitle removes the key that matches the given title from the current user's authorised keys.
func (gincl *Client) DeletePubKeyByTitle(title string) error {
	log.Write("Searching for key with title '%s'", title)
	keys, err := gincl.GetUserKeys()
	if err != nil {
		log.Write("Error when getting user keys: %v", err)
		return err
	}
	for _, key := range keys {
		if key.Title == title {
			return gincl.DeletePubKey(key.ID)
		}
	}
	return fmt.Errorf("No key with title '%s'", title)
}

// DeletePubKeyByIdx removes the key with the given index from the current user's authorised keys.
// Upon deletion, it returns the title of the key that was deleted.
// Note that the first key has index 1.
func (gincl *Client) DeletePubKeyByIdx(idx int) (string, error) {
	log.Write("Searching for key with index '%d'", idx)
	if idx < 1 {
		log.Write("Invalid index [idx %d]", idx)
		return "", fmt.Errorf("Invalid key index '%d'", idx)
	}
	log.Write("Searching for key with index '%d'", idx)
	keys, err := gincl.GetUserKeys()
	if err != nil {
		log.Write("Error when getting user keys: %v", err)
		return "", err
	}
	if idx > len(keys) {
		log.Write("Invalid index [idx %d > N %d]", idx, len(keys))
		return "", fmt.Errorf("Invalid key index '%d'", idx)
	}
	key := keys[idx-1]
	return key.Title, gincl.DeletePubKey(key.ID)
}

// Login requests a token from the auth server and stores the username and
// token to file and adds them to the Client.
// It also generates a key pair for the user for use in git commands.
// (See also NewToken)
func (gincl *Client) Login(username, password, clientID string) error {
	// retrieve user's active tokens
	tokens, err := gincl.GetTokens(username, password)
	if err != nil {
		return err
	}

	for _, token := range tokens {
		if token.Name == clientID {
			// found our token
			gincl.Username = username
			gincl.Token = token.Sha1
			log.Write("Found %s access token", clientID)
			break
		}
	}

	if len(gincl.Token) == 0 {
		// no existing token; creating new one
		log.Write("Requesting new token from server")
		err = gincl.NewToken(username, password, clientID)
		if err != nil {
			return err
		}
		log.Write("Login successful. Username: %s", username)
	}

	// Store token (to file)
	err = gincl.StoreToken(gincl.srvalias)
	if err != nil {
		return fmt.Errorf("Error while storing token: %s", err.Error())
	}

	gincl.web.SetToken(gincl.Token)

	// Make keys
	err = gincl.MakeSessionKey()
	if err != nil {
		return err
	}

	// making SSH config file for server
	khf, _ := GetKnownHosts()
	sshconflines := []string{
		"IdentitiesOnly yes",
		"StrictHostKeyChecking yes",
		fmt.Sprintf("UserKnownHostsFile %q", khf),
	}

	configpath, _ := config.Path(true)
	cfg := config.Read()
	for alias, srv := range cfg.Servers {
		keyfilepath := filepath.Join(configpath, fmt.Sprintf("%s.key", alias))
		idfile := fmt.Sprintf("IdentityFile %q", keyfilepath)
		sshconflines = append(sshconflines, fmt.Sprintf("Host %s", srv.Git.Host))
		sshconflines = append(sshconflines, idfile)
	}

	sshconf := strings.Join(sshconflines, "\n")
	sshconffile := filepath.Join(configpath, "ssh.conf")
	ioutil.WriteFile(sshconffile, []byte(sshconf), 0600)
	log.Write("SSH CONFIG:\n%s", sshconf)
	return nil
}

// GetTokens returns all the user's active access tokens from the GIN server.
func (gincl *Client) GetTokens(username, password string) ([]AccessToken, error) {
	fn := "GetTokens()"
	address := fmt.Sprintf("/api/v1/users/%s/tokens", username)
	res, err := gincl.web.GetBasicAuth(address, username, password)
	if err != nil {
		return nil, err // return error from GetBasicAuth directly
	}
	switch code := res.StatusCode; {
	case code == http.StatusInternalServerError:
		return nil, ginerror{UError: res.Status, Origin: fn, Description: "server error"}
	case code == http.StatusUnauthorized:
		return nil, ginerror{UError: res.Status, Origin: fn, Description: "authorisation failed"}
	case code != http.StatusOK:
		return nil, ginerror{UError: res.Status, Origin: fn} // Unexpected error
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	log.Write("Got response: %s", res.Status)
	tokens := []AccessToken{}
	err = json.Unmarshal(data, &tokens)
	if err != nil {
		return nil, ginerror{UError: err.Error(), Origin: fn, Description: "failed to parse response body"}
	}
	return tokens, nil
}

// NewToken requests a new user token from the GIN server and adds it to the
// Client along with the username.
func (gincl *Client) NewToken(username, password, clientID string) error {
	fn := "NewToken()"
	tokenCreate := &gogs.CreateAccessTokenOption{Name: clientID}
	address := fmt.Sprintf("/api/v1/users/%s/tokens", username)
	res, err := gincl.web.PostBasicAuth(address, username, password, tokenCreate)
	if err != nil {
		return err // return error from PostBasicAuth directly
	}
	switch code := res.StatusCode; {
	case code == http.StatusInternalServerError:
		return ginerror{UError: res.Status, Origin: fn, Description: "server error"}
	case code == http.StatusUnauthorized:
		return ginerror{UError: res.Status, Origin: fn, Description: "authorisation failed"}
	case code != http.StatusCreated:
		return ginerror{UError: res.Status, Origin: fn} // Unexpected error
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	log.Write("Got response: %s", res.Status)
	token := AccessToken{}
	err = json.Unmarshal(data, &token)
	if err != nil {
		return ginerror{UError: err.Error(), Origin: fn, Description: "failed to parse response body"}
	}
	gincl.Username = username
	gincl.Token = token.Sha1
	gincl.web.SetToken(gincl.Token)
	return nil
}

// Logout logs out the currently logged in user in 3 steps:
// 1. Remove the public key matching the current hostname from the server.
// 2. Delete the private key file from the local machine.
// 3. Delete the user token.
func (gincl *Client) Logout() {
	// 1. Delete public key
	hostname, err := os.Hostname()
	if err != nil {
		log.Write("Could not retrieve hostname")
		hostname = unknownhostname
	}

	currentkeyname := fmt.Sprintf("GIN Client: %s@%s", gincl.Username, hostname)
	err = gincl.DeletePubKeyByTitle(currentkeyname)
	if err != nil {
		log.Write(err.Error())
	}

	// 2. Delete private key
	privKeyFiles := PrivKeyPath()
	err = os.Remove(privKeyFiles[gincl.srvalias])
	if err != nil {
		log.Write("Error deleting key file")
	} else {
		log.Write("Private key file deleted")
	}

	err = DeleteToken(gincl.srvalias)
	if err != nil {
		log.Write("Error deleting token file")
	}
}

// DefaultServer returns the alias of the configured default gin server.
func DefaultServer() string {
	conf := config.Read()
	return conf.DefaultServer
}

// SetDefaultServer sets the alias of the default gin server.
// Returns with error if no server with the given alias exists.
func SetDefaultServer(alias string) error {
	conf := config.Read()
	if _, ok := conf.Servers[alias]; !ok {
		return fmt.Errorf("server with alias '%s' does not exist", alias)
	}
	config.SetDefaultServer(alias)
	return nil
}

// RemoveServer removes a server from the user configuration.
// Returns with error if no server with the given alias exists.
func RemoveServer(alias string) error {
	conf := config.Read()
	if _, ok := conf.Servers[alias]; !ok {
		return fmt.Errorf("server with alias '%s' does not exist", alias)
	}
	config.RmServerConf(alias)
	return nil
}
