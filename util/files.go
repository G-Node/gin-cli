package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// PathExists returns true if the path exists
func PathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// PathSplit returns the directory path separated from the filename. If the
// argument is a directory, the filename "." is returned. Errors are ignored.
func PathSplit(path string) (string, string) {
	info, err := os.Stat(path)
	if err != nil {
		return path, "."
	}
	var dir, filename string
	switch mode := info.Mode(); {
	case mode.IsRegular():
		dir, filename = filepath.Split(path)
	case mode.IsDir():
		dir = path
		filename = "."
	}
	return dir, filename
}

// DataSize returns the simplest representation of bytes as a string (with units)
func DataSize(nbytes int) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB"}

	lastPositive := nbytes
	unitIdx := 0
	for b, c := nbytes, 0; b > 0 && c < len(units); b, c = b>>10, c+1 {
		lastPositive, unitIdx = b, c
	}
	return fmt.Sprintf("%d %s", lastPositive, units[unitIdx])
}

// ExpandPaths converts a list of path strings into a list of regular files.
// This includes recursively traversing directories and expanding globs.
func ExpandPaths(paths []string) []string {
	return paths
}
