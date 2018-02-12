package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	LogWrite("Creating key pair")
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
	configpath, err := ConfigPath(false)
	if err != nil {
		LogWrite("Error getting user's config path. Can't load key file.")
		LogWrite(err.Error())
		return ""
	}
	return filepath.Join(configpath, fmt.Sprintf("%s.key", user))
}

// HostKeyPath returns the full path for the location of the gin host key file.
func HostKeyPath() string {
	configpath, err := ConfigPath(false)
	if err != nil {
		LogWrite("Error getting user's config path. Can't create host key file.")
		LogWrite(err.Error())
		return ""
	}
	return filepath.Join(configpath, "ginhostkey")
}

// GitSSHEnv returns the value that should be set for the GIT_SSH_COMMAND environment variable
// in order to use the user's private key.
func GitSSHEnv(user string) string {
	sshbin := Config.Bin.SSH
	// Windows git seems to require Unix paths for the SSH command -- this is dirty but works
	ossep := string(os.PathSeparator)
	sshbin = strings.Replace(sshbin, ossep, "/", -1)
	sshbin = strings.Replace(sshbin, " ", "\\ ", -1)
	keyfile := PrivKeyPath(user)
	keyfile = strings.Replace(keyfile, ossep, "/", -1)
	keyfile = strings.Replace(keyfile, " ", "\\ ", -1)
	gitSSHCmd := fmt.Sprintf("GIT_SSH_COMMAND=%s -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=no", sshbin, keyfile)
	LogWrite("env %s", gitSSHCmd)
	return gitSSHCmd
}
