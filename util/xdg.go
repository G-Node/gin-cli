// Simple utility functions for getting configuration paths based on the XDG specification.

package util

import (
	"fmt"
	"os"
	"path/filepath"
)

var suffix = "gin"

func makePath(path string) {
	err := os.MkdirAll(path, 0777)
	if err != nil {
		fmt.Printf("Error accessing directory %s\n", path)
		panic(err)
	}
}

// ConfigPath returns the configuration path where configuration files should be stored.
func ConfigPath() (path string) {
	// TODO: OS dependent paths
	xdghome := os.Getenv("XDG_CONFIG_HOME")
	homedir := os.Getenv("HOME")

	if xdghome != "" {
		path = filepath.Join(xdghome, suffix)
	} else {
		path = filepath.Join(homedir, ".config", suffix)
	}
	makePath(path)
	return path
}
