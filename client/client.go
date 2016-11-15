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
	"strings"

	"github.com/G-Node/gin-cli/util"
)

// Client struct for making requests
type Client struct {
	Host  string
	Token string
	web   *http.Client
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

// DoLogin Login with given username and password and store token
func (client *Client) DoLogin(username, password string) ([]byte, error) {
	params := url.Values{}
	params.Add("scope", "repo-read repo-write account-read account-write")
	params.Add("username", username)
	params.Add("password", password)
	params.Add("grant_type", "password")
	params.Add("client_id", "gin")
	params.Add("client_secret", "secret")

	address := fmt.Sprintf("%s/oauth/token", client.Host)

	req, _ := http.NewRequest("POST", address, strings.NewReader(params.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := client.web.Do(req)

	if err != nil {
		return nil, err
	} else if res.StatusCode != 200 {
		return nil, fmt.Errorf("[Login error] %s", res.Status)
	}

	defer CloseRes(res.Body)
	return ioutil.ReadAll(res.Body)
}

// Get Send a GET request
func (client *Client) Get(address string) (*http.Response, error) {
	requrl := urlJoin(client.Host, address)
	req, err := http.NewRequest("GET", requrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.Token))
	return client.web.Do(req)
}

// Post Send a POST request
func (client *Client) Post(address string, data interface{}) (*http.Response, error) {
	datajson, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	requrl := urlJoin(client.Host, address)
	req, err := http.NewRequest("POST", requrl, bytes.NewReader(datajson))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.Token))
	return client.web.Do(req)
}

// NewClient create new client for specific host
func NewClient(address string) *Client {
	return &Client{Host: address, web: &http.Client{}}
}

// CloseRes Close result buffer
func CloseRes(b io.ReadCloser) {
	err := b.Close()
	util.CheckErrorMsg(err, "Error during cleanup.")
}
