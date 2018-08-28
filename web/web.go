/*
Package web provides functions for interacting with a REST API.
It was designed to work with GIN Gogs (https://github.com/G-Node/gogs), a fork of the Gogs git service (https://github.com/gogits/gogs), and therefore only implements requests and assumes responses for working with that particular API.
Beyond that, the implementation is relatively general and service agnostic.
*/
package web

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/G-Node/gin-cli/git/shell"
	gogs "github.com/gogits/go-gogs-client"
)

// weberror alias to util.Error
type weberror = shell.Error

// UserToken struct for username and token
type UserToken struct {
	Username string
	Token    string
}

// Client struct for making requests
type Client struct {
	Host string
	UserToken
	web *http.Client
}

func urlJoin(parts ...string) string {
	// First part must be a valid URL
	u, err := url.Parse(parts[0])
	if err != nil {
		log.Write("Bad URL in urlJoin: %v", parts)
		return ""
	}

	for _, part := range parts[1:] {
		u.Path = path.Join(u.Path, part)
	}
	return u.String()
}

func parseServerError(err error) (errmsg string) {
	// should only receive non-nil error messages, but lets check anyway
	if err != nil {
		errmsg = err.Error()
		if strings.HasSuffix(errmsg, "connection refused") {
			errmsg = "server refused connection"
		} else if strings.HasSuffix(errmsg, "no such host") {
			errmsg = "server unreachable"
		} else if strings.HasSuffix(errmsg, "timeout") {
			errmsg = "request timed out"
		}
	}
	return
}

// Get sends a GET request to address.
// The address is appended to the client host, so it should be specified without a host prefix.
func (cl *Client) Get(address string) (*http.Response, error) {
	requrl := urlJoin(cl.Host, address)
	req, err := http.NewRequest("GET", requrl, nil)
	if err != nil {
		return nil, weberror{UError: err.Error(), Origin: fmt.Sprintf("Get(%s)", requrl)}
	}
	req.Header.Set("content-type", "application/jsonAuthorization")
	log.Write("Performing GET: %s", req.URL)
	if cl.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", cl.Token))
	}
	resp, err := cl.web.Do(req)
	if err != nil {
		return nil, weberror{UError: err.Error(), Origin: fmt.Sprintf("Get(%s)", requrl), Description: parseServerError(err)}
	}
	return resp, nil
}

// Post sends a POST request to address with the provided data.
// The address is appended to the client host, so it should be specified without a host prefix.
func (cl *Client) Post(address string, data interface{}) (*http.Response, error) {
	fn := fmt.Sprintf("Post(%s, <data>)", address)
	datajson, err := json.Marshal(data)
	if err != nil {
		return nil, weberror{UError: err.Error(), Origin: fn}
	}
	requrl := urlJoin(cl.Host, address)
	req, err := http.NewRequest("POST", requrl, bytes.NewReader(datajson))
	if err != nil {
		return nil, weberror{UError: err.Error(), Origin: fn}
	}
	req.Header.Set("content-type", "application/jsonAuthorization")
	if cl.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", cl.Token))
		log.Write("Added token to POST")
	}
	log.Write("Performing POST: %s", req.URL)
	resp, err := cl.web.Do(req)
	if err != nil {
		err = weberror{UError: err.Error(), Origin: fn, Description: parseServerError(err)}
	}
	return resp, err
}

// GetBasicAuth sends a GET request to address.
// The username and password are used to perform Basic authentication.
func (cl *Client) GetBasicAuth(address, username, password string) (*http.Response, error) {
	fn := fmt.Sprintf("GetBasicAuth(%s)", address)
	requrl := urlJoin(cl.Host, address)
	req, err := http.NewRequest("GET", requrl, nil)
	if err != nil {
		return nil, weberror{UError: err.Error(), Origin: fn}
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", gogs.BasicAuthEncode(username, password)))
	log.Write("Performing GET: %s", req.URL)
	resp, err := cl.web.Do(req)
	if err != nil {
		err = weberror{UError: err.Error(), Origin: fn, Description: parseServerError(err)}
	}
	return resp, err
}

// PostBasicAuth sends a POST request to address with the provided data.
// The username and password are used to perform Basic authentication.
func (cl *Client) PostBasicAuth(address, username, password string, data interface{}) (*http.Response, error) {
	fn := fmt.Sprintf("PostBasicAuth(%s)", address)
	datajson, err := json.Marshal(data)
	if err != nil {
		return nil, weberror{UError: err.Error(), Origin: fn}
	}
	requrl := urlJoin(cl.Host, address)
	req, err := http.NewRequest("POST", requrl, bytes.NewReader(datajson))
	if err != nil {
		return nil, weberror{UError: err.Error(), Origin: fn}
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", gogs.BasicAuthEncode(username, password)))
	log.Write("Performing POST: %s", req.URL)
	resp, err := cl.web.Do(req)
	if err != nil {
		err = weberror{UError: err.Error(), Origin: fn, Description: parseServerError(err)}
	}
	return resp, err
}

// Delete sends a DELETE request to address.
func (cl *Client) Delete(address string) (*http.Response, error) {
	fn := fmt.Sprintf("Delete(%s)", address)
	requrl := urlJoin(cl.Host, address)
	req, err := http.NewRequest("DELETE", requrl, nil)
	if err != nil {
		return nil, weberror{UError: err.Error(), Origin: fn}
	}
	req.Header.Set("content-type", "application/jsonAuthorization")
	if cl.Token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", cl.Token))
		log.Write("Added token to DELETE")
	}
	log.Write("Performing DELETE: %s", req.URL)
	resp, err := cl.web.Do(req)
	if err != nil {
		err = weberror{UError: err.Error(), Origin: fn, Description: parseServerError(err)}
	}
	return resp, err
}

// New creates a new client for a given host.
func New(host string) *Client {
	return &Client{Host: host, web: &http.Client{}}
}

// LoadToken reads the username and auth token from the token file and sets the
// values in the struct.
func (ut *UserToken) LoadToken(srvalias string) error {
	fn := fmt.Sprintf("LoadToken(%s)", srvalias)
	if ut.Username != "" && ut.Token != "" {
		return nil
	}
	path, _ := config.Path(false) // Error can only occur when create=True
	filename := fmt.Sprintf("%s.token", srvalias)
	filepath := filepath.Join(path, filename)
	log.Write("Loading token [server %s] %s", srvalias, filepath)
	file, err := os.Open(filepath)
	if err != nil {
		log.Write("Failed to load")
		return weberror{UError: err.Error(), Origin: fn, Description: "failed to load user token"}
	}
	defer closeFile(file)

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(ut)
	if err != nil {
		log.Write("Failed to parse")
		return weberror{UError: err.Error(), Origin: fn, Description: "failed to parse user token"}
	}
	return nil
}

// StoreToken saves the username and auth token to the token file.
func (ut *UserToken) StoreToken(srvalias string) error {
	fn := fmt.Sprintf("StoreToken(%s)", srvalias)
	path, err := config.Path(true)
	if err != nil {
		return weberror{UError: err.Error(), Origin: fn}
	}
	filename := fmt.Sprintf("%s.token", srvalias)
	filepath := filepath.Join(path, filename)
	log.Write("Saving token [server %s] %s", srvalias, filepath)
	file, err := os.Create(filepath)
	if err != nil {
		log.Write("Failed to create token file %s", filepath)
		return weberror{UError: err.Error(), Origin: fn, Description: fmt.Sprintf("failed to create token file %s", filepath)}
	}
	defer closeFile(file)

	encoder := gob.NewEncoder(file)
	err = encoder.Encode(ut)
	if err != nil {
		log.Write("Failed to write token to file %s", filepath)
		return weberror{UError: err.Error(), Origin: fn, Description: "failed to store token"}
	}
	log.Write("Saved")
	return nil
}

// DeleteToken deletes the token file if it exists (for finalising a logout).
func DeleteToken(srvalias string) error {
	path, _ := config.Path(false) // Error can only occur when create=True
	filename := fmt.Sprintf("%s.token", srvalias)
	tokenpath := filepath.Join(path, filename)
	err := os.Remove(tokenpath)
	if err != nil {
		return weberror{UError: err.Error(), Origin: "DeleteToken()", Description: "could not delete token"}
	}
	log.Write("Token deleted")
	return nil
}

// CloseRes closes a given result buffer (for use with defer).
func CloseRes(b io.ReadCloser) {
	b.Close()
}

func closeFile(f *os.File) {
	_ = f.Close()
}
