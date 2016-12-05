package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

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

// Get sends a GET request to address.
// The address is appended to the client host, so it should be specified without a host prefix.
func (client *Client) Get(address string) (*http.Response, error) {
	requrl := urlJoin(client.Host, address)
	req, err := http.NewRequest("GET", requrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.Token))
	return client.web.Do(req)
}

// Post sends a POST request to address with the provided data.
// The address is appended to the client host, so it should be specified without a host prefix.
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

// NewClient creates a new client for a given host.
func NewClient(address string) *Client {
	return &Client{Host: address, web: &http.Client{}}
}

// CloseRes closes a given result buffer (for use with defer).
func CloseRes(b io.ReadCloser) {
	err := b.Close()
	util.CheckErrorMsg(err, "Error during cleanup.")
}
