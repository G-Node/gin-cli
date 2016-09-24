package client

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

// Client struct for making requests
type Client struct {
	Host  string
	Token string
	web   *http.Client
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

	b, err := ioutil.ReadAll(res.Body)
	return b, err
}

// Get Send a GET request
func (client *Client) Get(address string) (*http.Response, error) {
	requrl := client.Host + address
	req, err := http.NewRequest("GET", requrl, nil)
	if err != nil {
		// TODO: Handle error
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", client.Token))
	res, err := client.web.Do(req)
	if err != nil {
		// TODO: Handle error
		return res, err
	}
	return res, err
}

// NewClient create new client for specific host
func NewClient(address string) *Client {
	return &Client{Host: address, web: &http.Client{}}
}

// CloseRes Close result buffer
func CloseRes(b io.ReadCloser) {
	err := b.Close()
	if err != nil {
		fmt.Println("Error during cleanup:", err)
	}
}
