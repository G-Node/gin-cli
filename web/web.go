/*
Package web provides functions for interacting with a REST API.
It was designed to work with GIN Gogs (https://github.com/G-Node/gogs), a fork of the Gogs git service (https://github.com/gogits/gogs), and therefore only implements requests and assumes responses for working with that particular API.
Beyond that, the implementation is relatively general and service agnostic.
*/
package web

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
)

// Client struct for making requests
type Client struct {
	Host  string
	token string
	web   *http.Client
}

// SetToken sets a token that will be used to authorise all subsequent web
// requests.
func (cl *Client) SetToken(token string) {
	cl.token = token
}

func urlJoin(parts ...string) (string, error) {
	// First part must be a valid URL
	u, err := url.Parse(parts[0])
	if err != nil {
		return "", fmt.Errorf("Bad URL(s) in join %q: %s", parts, err.Error())
	}

	for _, part := range parts[1:] {
		u.Path = path.Join(u.Path, part)
	}
	return u.String(), nil
}

func parseServerError(err error) error {
	// should only receive non-nil error messages, but lets check anyway
	if err == nil {
		return nil
	}
	errmsg := err.Error()
	if strings.HasSuffix(errmsg, "connection refused") {
		errmsg = "server refused connection"
	} else if strings.HasSuffix(errmsg, "no such host") {
		errmsg = "server unreachable"
	} else if strings.HasSuffix(errmsg, "timeout") {
		errmsg = "request timed out"
	}
	return fmt.Errorf(errmsg)
}

func basicauthenc(username, password string) string {
	userpass := fmt.Sprintf("%s:%s", username, password)
	return base64.StdEncoding.EncodeToString([]byte(userpass))
}

// Get sends a GET request to address.
// The address is appended to the client host, so it should be specified without a host prefix.
func (cl *Client) Get(address string) (*http.Response, error) {
	requrl, err := urlJoin(cl.Host, address)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", requrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/jsonAuthorization")
	if cl.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", cl.token))
	}
	resp, err := cl.web.Do(req)
	if err != nil {
		return nil, parseServerError(err)
	}
	return resp, nil
}

// Post sends a POST request to address with the provided data.
// The address is appended to the client host, so it should be specified without a host prefix.
func (cl *Client) Post(address string, data interface{}) (*http.Response, error) {
	datajson, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	requrl, err := urlJoin(cl.Host, address)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", requrl, bytes.NewReader(datajson))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/jsonAuthorization")
	if cl.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", cl.token))
	}
	resp, err := cl.web.Do(req)
	if err != nil {
		err = parseServerError(err)
	}
	return resp, err
}

// GetBasicAuth sends a GET request to address.
// The username and password are used to perform Basic authentication.
func (cl *Client) GetBasicAuth(address, username, password string) (*http.Response, error) {
	requrl, err := urlJoin(cl.Host, address)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", requrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", basicauthenc(username, password)))
	resp, err := cl.web.Do(req)
	if err != nil {
		err = parseServerError(err)
	}
	return resp, err
}

// PostBasicAuth sends a POST request to address with the provided data.
// The username and password are used to perform Basic authentication.
func (cl *Client) PostBasicAuth(address, username, password string, data interface{}) (*http.Response, error) {
	datajson, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	requrl, err := urlJoin(cl.Host, address)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", requrl, bytes.NewReader(datajson))
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", basicauthenc(username, password)))
	resp, err := cl.web.Do(req)
	if err != nil {
		err = parseServerError(err)
	}
	return resp, err
}

// Delete sends a DELETE request to address.
func (cl *Client) Delete(address string) (*http.Response, error) {
	requrl, err := urlJoin(cl.Host, address)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("DELETE", requrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/jsonAuthorization")
	if cl.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", cl.token))
	}
	resp, err := cl.web.Do(req)
	if err != nil {
		err = parseServerError(err)
	}
	return resp, err
}

// New creates a new client for a given host.
func New(host string) *Client {
	return &Client{Host: host, web: &http.Client{}}
}

// CloseRes closes a given result buffer (for use with defer).
func CloseRes(b io.ReadCloser) {
	b.Close()
}
