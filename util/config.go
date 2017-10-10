package util

import (
	"fmt"
	"os"
	"path"

	"github.com/spf13/viper"
)

type conf struct {
	AuthHost string
	RepoHost string
	GitHost  string
	GitUser  string
	Bin      struct {
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
	viper.SetDefault("auth.address", "https://web.gin.g-node.org")
	viper.SetDefault("auth.port", "443")
	viper.SetDefault("repo.address", "https://web.gin.g-node.org")
	viper.SetDefault("repo.port", "443")
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

	authAddress := viper.GetString("auth.address")
	authPort := viper.GetInt("auth.port")
	Config.AuthHost = fmt.Sprintf("%s:%d", authAddress, authPort)

	repoAddress := viper.GetString("repo.address")
	repoPort := viper.GetInt("repo.port")
	Config.RepoHost = fmt.Sprintf("%s:%d", repoAddress, repoPort)

	gitAddress := viper.GetString("git.address")
	gitPort := viper.GetInt("git.port")
	Config.GitHost = fmt.Sprintf("%s:%d", gitAddress, gitPort)

	Config.GitUser = viper.GetString("git.user")

	LogWrite("Configuration values")
	LogWrite("%+v", Config)

	return nil
}
