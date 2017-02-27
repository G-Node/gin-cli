package util

import (
	"fmt"

	"github.com/spf13/viper"
)

type conf struct {
	Bin struct {
		Git      string
		GitAnnex string
		SSH      string
	}
	Host struct {
		Auth struct {
			Address string
			Port    int
		}
		Repo struct {
			Address string
			Port    int
		}
		Git struct {
			Address string
			Port    int
			User    string
		}
	}
	Annex struct {
		Exclude []string
		MinSize string
	}
}

// Conf makes the configuration options available after LoadConfig is called
var Conf conf

// LoadConfig reads in the configuration and makes it available through the package globals
func LoadConfig() error {
	// Binaries
	viper.SetDefault("bin.git", "git")
	viper.SetDefault("bin.gitannex", "git-annex")
	viper.SetDefault("bin.ssh", "ssh")

	// Hosts
	viper.SetDefault("host.auth.address", "auth.gin.g-node.org")
	viper.SetDefault("host.auth.port", "443")
	viper.SetDefault("host.repo.address", "repo.gin.g-node.org")
	viper.SetDefault("host.repo.port", "443")
	viper.SetDefault("host.git.address", "gin.g-node.org")
	viper.SetDefault("host.git.port", "22")
	viper.SetDefault("host.git.user", "git")

	// annex filters
	viper.SetDefault("annex.exclude", [...]string{"md", "rst", "txt", "c", "cpp", "h", "hpp", "py", "go"})
	viper.SetDefault("annex.minsize", "10M")

	viper.SetConfigName("config")
	configpath, _ := ConfigPath(false)
	// Add /etc/gin/config ?
	viper.AddConfigPath(configpath)
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil {
		return fmt.Errorf("Error reading config file: %s", err.Error())
	}

	err = viper.Unmarshal(&Conf)
	if err != nil {
		return fmt.Errorf("Error reading config file: %s", err.Error())
	}

	return nil
}
