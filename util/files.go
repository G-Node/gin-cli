package util

import (
	"fmt"
	"os"
)

// PathExists returns true if the path exists
func PathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
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
