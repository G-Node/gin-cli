// Package config handles reading of the user configuration for the client.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/G-Node/gin-cli/ginclient/log"
	"github.com/fatih/color"
	"github.com/shibukawa/configdir"
	"github.com/spf13/viper"
)

var (
	configDirs = configdir.New("g-node", "gin")
	yellow     = color.New(color.FgYellow).SprintFunc()
)

// WebCfg is the configuration for the web server.
type WebCfg struct {
	Protocol string
	Host     string
	Port     uint16
}

// AddressStr constructs a full address string from the configuration.
// The string has the format Scheme://Host:Port (e.g., https://web.gin.g-node.org:443)
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

// GinCliCfg holds the client configuration values.
type GinCliCfg struct {
	Servers map[string]ServerCfg
	Bin     struct {
		Git      string
		GitAnnex string
		SSH      string
	}
	Annex struct {
		Exclude []string
		MinSize string
	}
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

// local configuration cache
var configuration GinCliCfg
var set = false

// Read loads in the configuration from the config file(s) and returns a populated GinConfiguration struct.
// The configuration is cached. Subsequent reads reuse the already loaded configuration.
func Read() GinCliCfg {
	if set {
		return configuration
	}
	viper.Reset()
	viper.SetTypeByDefaultValue(true)
	// Binaries
	viper.SetDefault("bin.git", "git")
	viper.SetDefault("bin.gitannex", "git-annex")
	viper.SetDefault("bin.ssh", "ssh")

	// annex filters
	viper.SetDefault("annex.minsize", "10M")

	// Merge in user config file
	confpath, _ := Path(false)
	configFileName := "config.yml"
	confpath = filepath.Join(confpath, configFileName)

	viper.SetConfigFile(confpath)
	cerr := viper.MergeInConfig()
	if cerr == nil {
		log.Write("Found config file %s", confpath)
	}

	// Servers
	servers := make(map[string]ServerCfg)

	// append default gin configuration
	servers["gin"] = ServerCfg{
		WebCfg{
			Protocol: "https",
			Host:     "web.gin.g-node.org",
			Port:     443,
		},
		GitCfg{
			Host:    "gin.g-node.org",
			Port:    22,
			User:    "git",
			HostKey: "gin.g-node.org,141.84.41.216 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBE5IBgKP3nUryEFaACwY4N3jlqDx8Qw1xAxU2Xpt5V0p9RNefNnedVmnIBV6lA3n+9kT1OSbyqA/+SgsQ57nHo0=",
		},
	}

	// Read the config, overwriting the "gin" configuration if it exists
	srvcfgMap := viper.GetStringMap("servers")
	for alias, cfg := range srvcfgMap {
		// Marshal map then unmarshal into ServerCfg struct
		marshaled, _ := json.Marshal(cfg)
		s := ServerCfg{}
		err := json.Unmarshal(marshaled, &s)
		if err != nil {
			fmt.Fprintf(color.Output, "%s invalid value found in configuration for '%s': server configuration ignored\n", yellow("[warning]"), alias)
			continue
		}
		servers[alias] = s
	}
	configuration.Servers = servers

	// Binaries
	configuration.Bin.Git = viper.GetString("bin.git")
	configuration.Bin.GitAnnex = viper.GetString("bin.gitannex")
	configuration.Bin.SSH = viper.GetString("bin.ssh")

	// configuration file in the repository root (annex excludes and size threshold only)
	reporoot, err := findreporoot(".")
	if err == nil {
		confpath := filepath.Join(reporoot, configFileName)
		viper.SetConfigFile(confpath)
		cerr = viper.MergeInConfig()
		if cerr == nil {
			log.Write("Found config file %s", confpath)
		}
	}
	configuration.Annex.Exclude = viper.GetStringSlice("annex.exclude")
	configuration.Annex.MinSize = viper.GetString("annex.minsize")

	log.Write("values")
	log.Write("%+v", configuration)

	// TODO: Validate URLs on config read
	set = true
	return configuration
}

func WriteServerConf(alias string, conf ServerCfg) error {
	fmt.Printf("Saving to %s: %+v", alias, conf)
	return nil
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
