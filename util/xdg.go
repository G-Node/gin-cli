// Simple utility functions for getting configuration paths based on the XDG specification.

package util

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

var suffix = "gin"

// ConfigPath returns the configuration path where configuration files should be stored.
func ConfigPath(create bool) (string, error) {
	// TODO: Handle Windows paths
	var path string
	var err error

	xdgconfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgconfig != "" {
		path = filepath.Join(xdgconfig, suffix)
	} else {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("Error getting user home dir")
		}
		homedir := usr.HomeDir
		path = filepath.Join(homedir, ".config", suffix)
	}

	if create {
		err = os.MkdirAll(path, 0777)
		if err != nil {
			return "", fmt.Errorf("Error creating directory %s", path)
		}
	}
	return path, err
}
