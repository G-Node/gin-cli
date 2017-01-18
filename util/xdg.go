// Simple utility functions for getting configuration paths based on the XDG specification.

package util

import (
	"fmt"
	"os"
	"path/filepath"
)

var suffix = "gin"

func makePath(path string) error {
	err := os.MkdirAll(path, 0777)
	if err != nil {
		return fmt.Errorf("Error accessing directory %s\n", path)
	}
	return nil
}

// ConfigPath returns the configuration path where configuration files should be stored.
func ConfigPath(create bool) (string, error) {
	// TODO: OS dependent paths
	xdghome := os.Getenv("XDG_CONFIG_HOME")
	homedir := os.Getenv("HOME")

	var path string
	var err error

	if xdghome != "" {
		path = filepath.Join(xdghome, suffix)
	} else {
		path = filepath.Join(homedir, ".config", suffix)
	}
	if create {
		err = makePath(path)
		if err != nil {
			return "", err
		}
	}
	return path, err
}
