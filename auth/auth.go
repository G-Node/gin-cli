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

// Client struct for making requests
type Client struct {
	Host string
	web  *http.Client
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

func storeToken(username string, token string) error {
	userTokenStr := username + "\n" + token

	err := ioutil.WriteFile("token", []byte(userTokenStr), 0600)

	if err != nil {
		return err
	}

	return nil
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

	client := NewClient("https://auth.gin.g-node.org")
	b, err := client.doLogin(username, password)
	var authresp proto.TokenResponse
	err = json.Unmarshal(b, &authresp)

	if err != nil {
		return err
	}

	err = storeToken(username, authresp.AccessToken)

	if err != nil {
		// Login success but unable to store token in file. Print error.
		return err
	}

	fmt.Printf("[Login success] You are now logged in as %s\n", username)
	fmt.Printf("You have been granted the following permissions: %v\n", strings.Replace(authresp.Scope, " ", ", ", -1))

	return nil

}
