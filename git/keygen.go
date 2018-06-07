package git

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
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

// PrivKeyPath returns a map with the full path for all the currently available private key files indexed by the server alias for each key.
func PrivKeyPath() map[string]string {
	configpath, err := config.Path(false)
	if err != nil {
		log.Write("Error getting user's config path. Can't load key file.")
		log.Write(err.Error())
		return nil
	}
	servers := config.Read().Servers
	keys := make(map[string]string)
	for srvalias := range servers {
		keyfilepath := filepath.Join(configpath, fmt.Sprintf("%s.key", srvalias))
		if pathExists(keyfilepath) {
			keys[srvalias] = keyfilepath
		}
	}
	return keys
}

// GetHostKey takes a git server configuration, queries the server via SSH, and returns the public key of the host (in the format required for the known_hosts file) and the key fingerprint.
func GetHostKey(gitconf config.GitCfg) (hostkeystr, fingerprint string, err error) {
	// HostKeyCallback constructs the keystring
	keycb := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		hostkeystr = fmt.Sprintf("%s", gitconf.Host)
		if gitconf.Port != 22 {
			// Only specify if non-standard port
			hostkeystr = fmt.Sprintf("%s:%d", hostkeystr, gitconf.Port)
		}
		ip := remote.String()
		if strings.HasSuffix(ip, ":22") {
			// Only specify if non-standard port
			ip = strings.TrimSuffix(ip, ":22")
		}
		hostkeystr = fmt.Sprintf("%s,%s %s", hostkeystr, ip, string(ssh.MarshalAuthorizedKey(key)))
		fingerprint = ssh.FingerprintSHA256(key)
		return nil
	}

	sshcon := ssh.ClientConfig{
		User:            gitconf.User,
		HostKeyCallback: keycb,
	}
	_, derr := ssh.Dial("tcp", fmt.Sprintf("%s:%d", gitconf.Host, gitconf.Port), &sshcon)
	if derr != nil && !strings.Contains(derr.Error(), "unable to authenticate") {
		// Other errors (auth error in particular) should be ignored
		err = fmt.Errorf("connection test failed: %s", derr)
	}
	return
}

// HostKeyPath returns the full path for the location of the gin host key file.
func HostKeyPath() string {
	configpath, err := config.Path(false)
	if err != nil {
		log.Write("Error getting user's config path. Can't create host key file.")
		log.Write(err.Error())
		return ""
	}
	defserver := config.Read().DefaultServer
	filename := fmt.Sprintf("%s.hostkey", defserver)
	return filepath.Join(configpath, filename)
}

// sshEnv returns the value that should be set for the GIT_SSH_COMMAND environment variable
// in order to use the user's private keys.
// The returned string contains all available private keys.
func sshEnv() string {
	// Windows git seems to require Unix paths for the SSH command -- this is dirty but works
	fixpathsep := func(p string) string {
		p = filepath.ToSlash(p)
		p = strings.Replace(p, " ", "\\ ", -1)
		return p
	}
	config := config.Read()
	sshbin := fixpathsep(config.Bin.SSH)
	keys := PrivKeyPath()
	keyargs := make([]string, len(keys))
	idx := 0
	for _, k := range keys {
		keyargs[idx] = fmt.Sprintf("-i %s", fixpathsep(k))
		idx++
	}
	keystr := strings.Join(keyargs, " ")
	hostkeyfile := HostKeyPath()
	gitSSHCmd := fmt.Sprintf("GIT_SSH_COMMAND=%s %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o 'UserKnownHostsFile=\"%s\"'", sshbin, keystr, hostkeyfile)
	log.Write("env %s", gitSSHCmd)
	return gitSSHCmd
}
