package git

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/G-Node/gin-cli/ginclient/config"
	"github.com/G-Node/gin-cli/ginclient/log"
	"golang.org/x/crypto/ssh"
)

// KeyPair simply holds a private-public key pair as strings, with no extra information.
type KeyPair struct {
	Private string
	Public  string
}

// KeyFile holds the absolute path and filename of the current logged in user's key file.
type KeyFile struct {
	Dir      string
	FileName string
	Active   bool
}

// MakeKeyPair generates and returns a private-public key pair.
func MakeKeyPair() (*KeyPair, error) {
	log.Write("Creating key pair")
	privkey, err := rsa.GenerateKey(rand.Reader, 2048) // TODO: Key size as parameter
	if err != nil {
		return nil, fmt.Errorf("Error generating key pair: %s", err)
	}

	// Private key to string
	privkeyDer := x509.MarshalPKCS1PrivateKey(privkey)
	privBlk := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privkeyDer,
	}
	privStr := string(pem.EncodeToMemory(&privBlk))

	// Public key to string
	pubkey, err := ssh.NewPublicKey(&privkey.PublicKey)
	if err != nil {
		return nil, err
	}
	pubStr := string(ssh.MarshalAuthorizedKey(pubkey))

	return &KeyPair{privStr, pubStr}, nil
}

// PrivKeyPath returns the full path for the location of the user's private key file.
func PrivKeyPath(user string) string {
	configpath, err := config.Path(false)
	if err != nil {
		log.Write("Error getting user's config path. Can't load key file.")
		log.Write(err.Error())
		return ""
	}
	return filepath.Join(configpath, fmt.Sprintf("%s.key", user))
}

// HostKeyPath returns the full path for the location of the gin host key file.
func HostKeyPath() string {
	configpath, err := config.Path(false)
	if err != nil {
		log.Write("Error getting user's config path. Can't create host key file.")
		log.Write(err.Error())
		return ""
	}
	return filepath.Join(configpath, "ginhostkey")
}

// GitSSHEnv returns the value that should be set for the GIT_SSH_COMMAND environment variable
// in order to use the user's private key.
func GitSSHEnv(user string) string {
	// Windows git seems to require Unix paths for the SSH command -- this is dirty but works
	fixpathsep := func(p string) string {
		p = filepath.ToSlash(p)
		p = strings.Replace(p, " ", "\\ ", -1)
		return p
	}
	config := config.Read()
	sshbin := fixpathsep(config.Bin.SSH)
	keyfile := fixpathsep(PrivKeyPath(user))
	// hostkeyfile := fixpathsep(HostKeyPath())
	hostkeyfile := HostKeyPath()
	gitSSHCmd := fmt.Sprintf("GIT_SSH_COMMAND=%s -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o 'UserKnownHostsFile=\"%s\"'", sshbin, keyfile, hostkeyfile)
	log.Write("env %s", gitSSHCmd)
	return gitSSHCmd
}
