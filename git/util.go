package git

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/G-Node/gin-cli/ginclient/log"
	humanize "github.com/dustin/go-humanize"
)

// General utility functions for the git and git-annex shell commands and their output.

var annexmodecache = make(map[string]bool)

func makeFileList(header string, fnames []string) string {
	if len(fnames) == 0 {
		return ""
	}
	var filelist bytes.Buffer
	_, _ = filelist.WriteString(fmt.Sprintf("%s (%d)\n", header, len(fnames)))
	for idx, name := range fnames {
		_, _ = filelist.WriteString(fmt.Sprintf("  %d: %s\n", idx+1, name))
	}
	_, _ = filelist.WriteString("\n")
	return filelist.String()
}

func calcRate(dbytes int, dt time.Duration) string {
	dtns := dt.Nanoseconds()
	if dtns <= 0 || dbytes <= 0 {
		return ""
	}
	rate := int64(dbytes) * 1000000000 / dtns
	return fmt.Sprintf("%s/s", humanize.IBytes(uint64(rate)))
}

func logstd(out, err []byte) {
	log.Write("[stdout]\n%s\n[stderr]\n%s", string(out), string(err))
}

func cutline(b []byte) (string, bool) {
	idx := -1
	cridx := bytes.IndexByte(b, '\r')
	nlidx := bytes.IndexByte(b, '\n')
	if cridx > 0 {
		idx = cridx
	} else {
		cridx = len(b) + 1
	}
	if nlidx > 0 && nlidx < cridx {
		idx = nlidx
	}
	if idx <= 0 {
		return string(b), true
	}
	return string(b[:idx]), false
}

// pathExists returns true if the path exists
func pathExists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

func stringInSlice(element string, strlist []string) bool {
	// TODO: Replace this function with a map where possible
	for _, str := range strlist {
		if element == str {
			return true
		}
	}
	return false
}

// filterpaths takes path descriptions (full paths, relative paths, files, or directories) and returns a slice of filepaths (as strings) excluding the files listed in excludes.
func filterpaths(paths, excludes []string) (filtered []string) {

	// walker adds files to filtered if they don't match excludes
	walker := func(path string, info os.FileInfo, err error) error {
		if filepath.Base(path) == ".git" {
			return filepath.SkipDir
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if !stringInSlice(path, excludes) {
			filtered = append(filtered, path)
		}
		return nil
	}

	for _, p := range paths {
		err := filepath.Walk(p, walker)
		if err != nil {
			log.Write("Error occured during path filtering: %s", err.Error())
		}
	}

	return
}

// CopyFile copies the contents of src file into dest file.
func CopyFile(src, dest string) error {
	if !pathExists(src) {
		return fmt.Errorf("source file '%s' does not exist", src)
	}
	if pathExists(dest) {
		return fmt.Errorf("destination file '%s' exists; refusing to overwrite", dest)
	}

	// set up read buffer
	srcfile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file for copy: %v", err.Error())
	}
	defer srcfile.Close()
	reader := bufio.NewReader(srcfile)

	// set up write buffer
	destfile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("failed to create destination file for copy: %v", err.Error())
	}
	defer destfile.Close()
	writer := bufio.NewWriter(destfile)
	defer writer.Flush()

	if _, err := io.Copy(writer, reader); err != nil {
		return fmt.Errorf("file copy failed: %v", err.Error())
	}

	return nil
}

// TODO: Move CopyFile and pathExists to a new subpackage: ginclient/ginutil or simply ginutil
