package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// KeyPair simply holds a private-public key pair as strings, with no extra information.
type KeyPair struct {
	Private string
	Public  string
}

// TempFile holds the absolute path and filename of a temporary private key.
type TempFile struct {
	Dir      string
	Filename string
	Active   bool
}

// MakeKeyPair generates and returns a private-public key pair.
func MakeKeyPair() (*KeyPair, error) {
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

// Temporary key file handling

// MakeTempFile creates a temporary file for storing temporary private keys.
// A TempFile struct is returned which holds the location and name of the new file.
func MakeTempFile(filename string) (TempFile, error) {
	dir, err := ioutil.TempDir("", "gin")
	if err != nil {
		return TempFile{}, fmt.Errorf("Error creating temporary key directory: %s", err.Error())
	}

	newfile := TempFile{Dir: dir, Filename: filename}
	return newfile, nil
}

// SaveTempKeyFile stores a given private key to a temporary file.
// Returns a TempFile struct which contains the absolute path and filename.
func SaveTempKeyFile(key string) (*TempFile, error) {
	dir, err := ioutil.TempDir("", "gin")
	if err != nil {
		return nil, fmt.Errorf("Error creating temporary key directory: %s", err)
	}
	newfile := TempFile{
		Dir:      dir,
		Filename: "priv",
	}
	if err != nil {
		return nil, err
	}
	err = newfile.Write(key)
	if err != nil {
		return nil, err
	}
	return &newfile, nil

}

// Write a string to the temporary file.
func (tf TempFile) Write(content string) error {
	if err := ioutil.WriteFile(tf.FullPath(), []byte(content), 0600); err != nil {
		return fmt.Errorf("Error writing temporary file: %s", err)
	}
	return nil
}

// Delete the temporary file and its diirectory.
func (tf TempFile) Delete() {
	_ = os.RemoveAll(tf.Dir)
	tf.Active = false
}

// FullPath returns the full path to the temporary file.
func (tf TempFile) FullPath() string {
	return filepath.Join(tf.Dir, tf.Filename)
}

// SSHOptString returns a formatted string that can be used in git-annex commands that should
// make use of the temporary private key.
func (tf TempFile) SSHOptString() string {
	return fmt.Sprintf("annex.ssh-options=-i %s", tf.FullPath())
}
