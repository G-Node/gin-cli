package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/G-Node/gin-auth/proto"
	"github.com/howeyc/gopass"
)

const authhost = "https://auth.gin.g-node.org"

// Client struct for making requests
type Client struct {
	Host  string
	Token string
	web   *http.Client
}

// NewClient create new client for specific host
func NewClient(address string) *Client {
	return &Client{Host: address, web: &http.Client{}}
}

func closeRes(b io.ReadCloser) {
	err := b.Close()
	if err != nil {
		fmt.Println("Error during cleanup:", err)
	}
}

func storeToken(token string) error {
	err := ioutil.WriteFile("token", []byte(token), 0600)

	if err != nil {
		return err
	}

	return nil
}

// LoadToken Get the current signed in username and auth token
func LoadToken(warn bool) (string, string) {

	tokenBytes, err := ioutil.ReadFile("token")
	tokenInfo := proto.TokenInfo{}
	var username, token string

	if err == nil {
		token = string(tokenBytes)
	} else {
		if warn {
			fmt.Println("You are not logged in.")
		}
		return "", ""
	}

	client := NewClient(authhost)
	res, err := client.Get("/oauth/validate/" + token)

	defer closeRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", ""
	}

	err = json.Unmarshal(b, &tokenInfo)

	username = tokenInfo.Login
	if username == "" && warn {
		// Token invalid: Delete file?
		fmt.Println("You are not logged in.")
		token = ""
	}
	return username, token
}

func (client *Client) doLogin(username, password string) ([]byte, error) {
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

	defer closeRes(res.Body)

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

// Login Request credentials, perform login, and store token
func Login(userarg interface{}) error {

	var username, password string

	if userarg == nil {
		// prompt for login
		fmt.Print("Login: ")
		fmt.Scanln(&username)
	} else {
		username = userarg.(string)
	}

	// prompt for password
	password = ""
	fmt.Print("Password: ")
	pwbytes, err := gopass.GetPasswdMasked()
	fmt.Println()
	if err != nil {
		// read error or gopass.ErrInterrupted
		if err == gopass.ErrInterrupted {
			fmt.Println("Cancelled.")
			return err
		}
		if err == gopass.ErrMaxLengthExceeded {
			fmt.Println("Error: Input too long.")
			return err
		}
	}

	password = string(pwbytes)

	if password == "" {
		fmt.Println("No password provided. Aborting.")
		return err
	}

	client := NewClient(authhost)
	b, err := client.doLogin(username, password)
	var authresp proto.TokenResponse
	err = json.Unmarshal(b, &authresp)

	if err != nil {
		return err
	}

	err = storeToken(authresp.AccessToken)

	if err != nil {
		// Login success but unable to store token in file. Print error.
		return err
	}

	fmt.Printf("[Login success] You are now logged in as %s\n", username)
	fmt.Printf("You have been granted the following permissions: %v\n", strings.Replace(authresp.Scope, " ", ", ", -1))

	return nil

}

// RequestAccount requests a specific account by name
func RequestAccount(name, token string) (proto.Account, error) {
	var acc proto.Account

	client := NewClient(authhost)
	client.Token = token
	res, err := client.Get("/api/accounts/" + name)

	if err != nil {
		fmt.Printf("[Error] Request failed: %s\n", err)
		return acc, err
	} else if res.StatusCode != 200 {
		return acc, fmt.Errorf("[Account search error] Server returned: %s", res.Status)
	}

	defer closeRes(res.Body)

	b, err := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(b, &acc)
	if err != nil {
		return acc, err
	}
	return acc, nil
}

// SearchAccount Search for account
func SearchAccount(query string) ([]proto.Account, error) {
	var results []proto.Account

	params := url.Values{}
	params.Add("q", query)
	url := fmt.Sprintf("%s/api/accounts?%s", authhost, params.Encode())
	client := NewClient(authhost)
	res, err := client.Get(url)

	if err != nil {
		return results, err
	} else if res.StatusCode != 200 {
		return results, fmt.Errorf("[Account search error] Server returned: %s", res.Status)
	}

	body, _ := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(body, &results)

	return results, nil
}
