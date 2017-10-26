package util

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/spf13/viper"
)

type conf struct {
	GinHost string
	GitHost string
	GitUser string
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

// Config makes the configuration options available after LoadConfig is called
var Config conf

// LoadConfig reads in the configuration and makes it available through Config package global
func LoadConfig() error {
	viper.SetTypeByDefaultValue(true)
	// Binaries
	viper.SetDefault("bin.git", "git")
	viper.SetDefault("bin.gitannex", "git-annex")
	viper.SetDefault("bin.ssh", "ssh")

	// Hosts
	// Disabling default gin.address for now and handling it manually in order to present deprecation message
	// viper.SetDefault("gin.address", "https://web.gin.g-node.org")
	viper.SetDefault("gin.port", "443")
	defaultHost := "https://web.gin.g-node.org"

	viper.SetDefault("git.address", "gin.g-node.org")
	viper.SetDefault("git.port", "22")
	viper.SetDefault("git.user", "git")

	// annex filters
	viper.SetDefault("annex.minsize", "10M")

	viper.SetConfigName("config")
	// prioritise config files in the directory of the executable
	// this is useful for portable packaging
	execloc, err := os.Executable()
	execpath, _ := path.Split(execloc)
	if err == nil {
		viper.AddConfigPath(execpath)
		LogWrite("Config path added %s", execpath)
	}

	configpath, _ := ConfigPath(false)
	viper.AddConfigPath(configpath)
	LogWrite("Config path added %s", configpath)

	err = viper.ReadInConfig()
	if err != nil {
		if !strings.Contains(err.Error(), "Not Found") {
			LogError(err)
		}
	}
	LogError(err)
	fileused := viper.ConfigFileUsed()
	if fileused != "" {
		LogWrite("Loading config file %s", viper.ConfigFileUsed())
	} else {
		LogWrite("No config file found. Using defaults.")
	}

	Config.Bin.Git = viper.GetString("bin.git")
	Config.Bin.GitAnnex = viper.GetString("bin.gitannex")
	Config.Bin.SSH = viper.GetString("bin.ssh")
	Config.Annex.Exclude = viper.GetStringSlice("annex.exclude")
	Config.Annex.MinSize = viper.GetString("annex.minsize")

	var oldConfigHost string
	if viper.IsSet("auth.address") || viper.IsSet("auth.port") {
		fmt.Fprintln(os.Stderr, "Auth server address configuration is no longer used. Use gin.address and gin.port instead.")
		oldConfigHost = fmt.Sprintf("%s:%d", viper.GetString("auth.address"), viper.GetInt("auth.port"))
	}

	if viper.IsSet("repo.address") || viper.IsSet("repo.port") {
		fmt.Fprintln(os.Stderr, "Repo server address configuration is no longer used. Use gin.address and gin.port instead.")
		oldConfigHost = fmt.Sprintf("%s:%d", viper.GetString("repo.address"), viper.GetInt("repo.port"))
	}

	// If the gin host is set use it. If it's not but an old config value is set, use that.
	// If neither is set, use the bulit-in default.
	if viper.IsSet("gin.address") {
		ginAddress := viper.GetString("gin.address")
		ginPort := viper.GetInt("gin.port")
		Config.GinHost = fmt.Sprintf("%s:%d", ginAddress, ginPort)
	} else if oldConfigHost != "" {
		Config.GinHost = oldConfigHost
		fmt.Fprintf(os.Stderr, "Using deprecated configuration value: %s. This will change in a future release.\n", oldConfigHost)
	} else {
		Config.GinHost = fmt.Sprintf("%s:%d", defaultHost, viper.GetInt("gin.port"))
	}

	gitAddress := viper.GetString("git.address")
	gitPort := viper.GetInt("git.port")
	Config.GitHost = fmt.Sprintf("%s:%d", gitAddress, gitPort)

	Config.GitUser = viper.GetString("git.user")

	LogWrite("Configuration values")
	LogWrite("%+v", Config)

	return nil
}
