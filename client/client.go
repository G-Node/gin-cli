package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"path/filepath"

	"github.com/G-Node/gin-cli/util"
	"github.com/G-Node/gin-core/gin"
)

// Client struct for making requests
type Client struct {
	Host     string
	Token    string
	Username string
	web      *http.Client
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

// Get sends a GET request to address.
// The address is appended to the client host, so it should be specified without a host prefix.
func (cl *Client) Get(address string) (*http.Response, error) {
	requrl := urlJoin(cl.Host, address)
	req, err := http.NewRequest("GET", requrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cl.Token))
	return cl.web.Do(req)
}

// Post sends a POST request to address with the provided data.
// The address is appended to the client host, so it should be specified without a host prefix.
func (cl *Client) Post(address string, data interface{}) (*http.Response, error) {
	datajson, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	requrl := urlJoin(cl.Host, address)
	req, err := http.NewRequest("POST", requrl, bytes.NewReader(datajson))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cl.Token))
	return cl.web.Do(req)
}

// NewClient creates a new client for a given host.
func NewClient(address string) *Client {
	return &Client{Host: address, web: &http.Client{}}
}

// LoadToken loads the auth token from the token file, checks it against the auth server,
// and sets the token and username in the auth struct.
func (cl *Client) LoadToken() error {
	tokenfile := filepath.Join(util.ConfigPath(), "token")
	tokenBytes, err := ioutil.ReadFile(tokenfile)
	tokenInfo := gin.TokenInfo{}

	if err != nil {
		return err
	}

	token := string(tokenBytes)
	res, err := cl.Get("/oauth/validate/" + token)
	if err != nil {
		// fmt.Fprintln(os.Stderr, "[Auth error] Error communicating with server.")
		return err
	}

	defer CloseRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, &tokenInfo)

	username := tokenInfo.Login
	if username == "" {
		return fmt.Errorf("[Auth error] You are not logged in")
	}
	cl.Username = username
	cl.Token = token
	return nil
}

// CloseRes closes a given result buffer (for use with defer).
func CloseRes(b io.ReadCloser) {
	err := b.Close()
	util.CheckErrorMsg(err, "Error during cleanup.")
}
