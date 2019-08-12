// Package config handles reading of the user configuration for the client.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/G-Node/gin-cli/gincmd/ginerrors"
	"github.com/fatih/color"
	"github.com/shibukawa/configdir"
	"github.com/spf13/viper"
)

const (
	defaultFileName = "config.yml"
)

var (
	configDirs = configdir.New("g-node", "gin")
	yellow     = color.New(color.FgYellow).SprintFunc()

	ginDefaultServer = ServerCfg{
		WebCfg{
			Protocol: "https",
			Host:     "gin.g-node.org",
			Port:     443,
		},
		GitCfg{
			Host:    "gin.g-node.org",
			Port:    22,
			User:    "git",
			HostKey: "gin.g-node.org,141.84.41.219 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBE5IBgKP3nUryEFaACwY4N3jlqDx8Qw1xAxU2Xpt5V0p9RNefNnedVmnIBV6lA3n+9kT1OSbyqA/+SgsQ57nHo0=",
		},
	}

	defaultConf = map[string]interface{}{
		// Binaries
		"bin.git":          "git",
		"bin.gitannex":     "git-annex",
		"gin.gitannexpath": "",
		"bin.ssh":          "ssh",
		// Annex filters
		"annex.minsize": "10M",
		"servers.gin":   ginDefaultServer,
		"defaultserver": "gin",
	}

	// configuration cache: used to avoid rereading during a single command invocation
	configuration GinCliCfg
	set           = false
)

// Types

// WebCfg is the configuration for the web server.
type WebCfg struct {
	Protocol string
	Host     string
	Port     uint16
}

// AddressStr constructs a full address string from the configuration.
// The string has the format Scheme://Host:Port (e.g., https://gin.g-node.org:443)
func (c WebCfg) AddressStr() string {
	return fmt.Sprintf("%s://%s:%d", c.Protocol, c.Host, c.Port)
}

// GitCfg is the configuration for the git server.
type GitCfg struct {
	User    string
	Host    string
	Port    uint16
	HostKey string
}

// AddressStr constructs a full address string from the configuration.
// The string has the format ssh://User@Host:Port (e.g., ssh://git@gin.g-node.org:22)
func (c GitCfg) AddressStr() string {
	return fmt.Sprintf("ssh://%s@%s:%d", c.User, c.Host, c.Port)
}

// ServerCfg holds the information required for GIN servers (web and git).
type ServerCfg struct {
	Web WebCfg
	Git GitCfg
}

// BinCfg holds the paths to the external binaries that the client depends on.
type BinCfg struct {
	Git          string
	GitAnnex     string
	GitAnnexPath string
	SSH          string
}

// AnnexCfg holds the configuration options for Git Annex (filtering rules).
type AnnexCfg struct {
	Exclude []string
	MinSize string
}

// GinCliCfg holds the client configuration values.
type GinCliCfg struct {
	Servers       map[string]ServerCfg
	DefaultServer string
	Bin           BinCfg
	Annex         AnnexCfg
}

// Read loads in the configuration from the config file(s), merges any defined values into the default configuration, and returns a populated GinConfiguration struct.
// The configuration is cached. Subsequent reads reuse the already loaded configuration.
func Read() GinCliCfg {
	if set {
		return configuration
	}
	viper.Reset()
	viper.SetTypeByDefaultValue(true)

	for k, v := range defaultConf {
		viper.SetDefault(k, v)
	}

	// Merge in user config file
	confpath, _ := Path(false)
	confpath = filepath.Join(confpath, defaultFileName)
	viper.SetConfigFile(confpath)
	cerr := viper.MergeInConfig()
	if cerr == nil {
		log.Write("Found config file %s", confpath)
	}

	viper.Unmarshal(&configuration)

	removeInvalidServerConfs()

	// configuration file in the repository root (annex excludes and size threshold only)
	reporoot, err := findreporoot(".")
	if err == nil {
		confpath := filepath.Join(reporoot, defaultFileName)
		viper.SetConfigFile(confpath)
		cerr = viper.MergeInConfig()
		if cerr == nil {
			log.Write("Found config file %s", confpath)
		}
	}
	configuration.Annex.Exclude = viper.GetStringSlice("annex.exclude")
	configuration.Annex.MinSize = viper.GetString("annex.minsize")

	// if Bin.GitAnnex is set but Bin.GitAnnexPath is not, set the path
	if configuration.Bin.GitAnnexPath == "" && configuration.Bin.GitAnnex != "" {
		path, _ := filepath.Split(configuration.Bin.GitAnnex)
		configuration.Bin.GitAnnexPath = path
	}

	set = true
	return configuration
}

func removeInvalidServerConfs() {
	// Check server configurations for invalid names and port numbers
	for alias := range viper.GetStringMap("servers") {
		if alias == "dir" {
			fmt.Fprintf(color.Error, "%s server alias '%s' is not allowed (reserved word): server configuration ignored\n", yellow("[warning]"), alias)
			delete(configuration.Servers, alias)
			continue
		}
		webport := viper.GetInt(fmt.Sprintf("servers.%s.web.port", alias))
		gitport := viper.GetInt(fmt.Sprintf("servers.%s.git.port", alias))
		if webport < 0 || webport > 65535 || gitport < 0 || gitport > 65535 {
			if alias == "gin" {
				fmt.Fprintf(color.Error, "%s invalid value found in configuration for '%s': using default\n", yellow("[warning]"), alias)
				configuration.Servers["gin"] = ginDefaultServer
			} else {
				fmt.Fprintf(color.Error, "%s invalid value found in configuration for '%s': server configuration ignored\n", yellow("[warning]"), alias)
				delete(configuration.Servers, alias)
			}
		}
	}
}

// SetConfig appends a key-value to the configuration file.  A useful
// utility function that loads the configuration only from the file, adds the
// new key-value pair, and saves it back, without loading the built-in
// defaults.  On successful write, the read cache is invalidated.
func SetConfig(key string, value interface{}) error {
	// Read in the file configuration ONLY
	confpath, err := Path(true) // create config path if necessary
	if err != nil {
		return err
	}
	confpath = filepath.Join(confpath, defaultFileName)
	v := viper.New()
	v.SetConfigFile(confpath)

	v.ReadInConfig()
	v.Set(key, value)
	v.WriteConfig()
	// invalidate the read cache
	set = false
	return nil
}

// AddServerConf writes a new server configuration into the user config file.
func AddServerConf(alias string, newcfg ServerCfg) error {
	key := fmt.Sprintf("servers.%s", alias)
	return SetConfig(key, newcfg)
}

// RmServerConf removes a server configuration from the user config file.
func RmServerConf(alias string) {
	var c GinCliCfg
	v := viper.New()
	// Merge in user config file
	confpath, _ := Path(false)
	confpath = filepath.Join(confpath, defaultFileName)
	v.SetConfigFile(confpath)
	v.ReadInConfig()
	v.Unmarshal(&c)
	delete(c.Servers, alias)
	v.Set("servers", c.Servers)
	v.WriteConfig()
	set = false
}

// SetDefaultServer writes the given name to the config file to server as the default server for web calls.
// An error is returned if the name doesn't exist in the current configuration.
func SetDefaultServer(alias string) {
	SetConfig("defaultserver", alias)
}

// Path returns the configuration path where configuration files should be stored.
// If the GIN_CONFIG_DIR environment variable is set, its value is returned, otherwise the platform default is used.
// If create is true and the directory does not exist, the full path is created.
func Path(create bool) (string, error) {
	confpath := os.Getenv("GIN_CONFIG_DIR")
	if confpath == "" {
		confpath = configDirs.QueryFolders(configdir.Global)[0].Path
	}
	var err error
	if create {
		err = os.MkdirAll(confpath, 0755)
		if err != nil {
			return "", fmt.Errorf("could not create config directory %s", confpath)
		}
	}
	return confpath, err
}

// Util functions //

// ParseWebString takes a string which contains all information about a
// server's web configuration (e.g., https://gin.g-node.org:443) and returns a
// WebCfg struct.
func ParseWebString(webstring string) (WebCfg, error) {
	var webconf WebCfg
	errmsg := fmt.Sprintf("invalid web configuration line %s", webstring)
	split := strings.SplitN(webstring, "://", 2)
	if len(split) != 2 {
		return webconf, fmt.Errorf("%s: %s", errmsg, ginerrors.MissingURLScheme)
	}
	webconf.Protocol = split[0]

	split = strings.SplitN(split[1], ":", 2)
	if len(split) != 2 {
		return webconf, fmt.Errorf(errmsg)
	}
	port, err := strconv.ParseUint(split[1], 10, 16)
	if err != nil {
		return webconf, fmt.Errorf("%s: %s", errmsg, ginerrors.BadPort)
	}
	webconf.Host, webconf.Port = split[0], uint16(port)
	return webconf, nil
}

// ParseGitString takes a string which contains all information about a
// server's git configuration (e.g., git@gin.g-node.org:22) and returns a
// GitCfg struct.
func ParseGitString(gitstring string) (GitCfg, error) {
	var gitconf GitCfg
	errmsg := fmt.Sprintf("invalid git configuration line %s", gitstring)
	split := strings.SplitN(gitstring, "@", 2)
	if len(split) != 2 {
		return gitconf, fmt.Errorf("%s: %s", errmsg, ginerrors.MissingGitUser)
	}
	gitconf.User = split[0]

	split = strings.SplitN(split[1], ":", 2)
	if len(split) != 2 {
		return gitconf, fmt.Errorf(errmsg)
	}
	port, err := strconv.ParseUint(split[1], 10, 16)
	if err != nil {
		return gitconf, fmt.Errorf("%s: %s", errmsg, ginerrors.BadPort)
	}
	gitconf.Host, gitconf.Port = split[0], uint16(port)
	return gitconf, nil
}

// pathExists returns true if the path exists
func pathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func findreporoot(path string) (string, error) {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		return "", err
	}
	gitdir := filepath.Join(path, ".git")
	if pathExists(gitdir) {
		return path, nil
	}
	updir := filepath.Dir(path)
	if updir == path {
		// root reached
		return "", fmt.Errorf("Not a repository")
	}

	return findreporoot(updir)
}
