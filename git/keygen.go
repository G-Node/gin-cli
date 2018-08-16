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
	"golang.org/x/crypto/ssh/knownhosts"
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

// GetHostKey takes a git server configuration, queries the server via SSH, and
// returns the public key of the host (in the format required for the
// known_hosts file) and the key fingerprint.
func GetHostKey(gitconf config.GitCfg) (hostkeystr, fingerprint string, err error) {
	// HostKeyCallback constructs the keystring
	keycb := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		addr := []string{hostname, remote.String()}
		hostkeystr = knownhosts.Line(addr, key)
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

// hostkeypath returns the full path for the location of the gin host key file.
func hostkeypath() string {
	configpath, _ := config.Path(false) // Error can only occur when attempting to create directory
	filename := "known_hosts"
	return filepath.Join(configpath, filename)
}

// WriteKnownHosts creates a known_hosts file in the config directory with all configured host keys.
func WriteKnownHosts() error {
	_, err := config.Path(true)
	if err != nil {
		log.Write("Failed to create config directory for known_hosts")
		return err
	}
	conf := config.Read()
	hostkeyfile := hostkeypath()
	f, err := os.OpenFile(hostkeyfile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		log.Write("Failed to create known_hosts")
		return err
	}
	defer f.Close()
	for _, srvcfg := range conf.Servers {
		_, err := f.WriteString(srvcfg.Git.HostKey + "\n")
		if err != nil {
			return err
		}
	}
	return nil
}

// GetKnownHosts returns the path to the known_hosts file.
// If the file does not exist it is created by calling WriteKnownHosts.
func GetKnownHosts() (string, error) {
	hkpath := hostkeypath()
	var err error
	if !pathExists(hkpath) {
		err = WriteKnownHosts()
	}
	return hkpath, err
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
	hostkeyfile, err := GetKnownHosts()
	var hfoptstr string
	if err == nil {
		hfoptstr = fmt.Sprintf("-o 'UserKnownHostsFile=\"%s\"'", hostkeyfile)
	}
	gitSSHCmd := fmt.Sprintf("GIT_SSH_COMMAND=%s %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes %s", sshbin, keystr, hfoptstr)
	log.Write("env %s", gitSSHCmd)
	return gitSSHCmd
}
