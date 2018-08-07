// Package log handles logging for the client.
package log

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/shibukawa/configdir"
)

var logfile *os.File
var logger *log.Logger

var configDirs = configdir.New("g-node", "gin")

const loglimit = 1048576 // 1 MiB

// trim reduces the size of a file to `loglimit`.
// It reads the contents and writes them back, removing the initial bytes to fit the limit.
// If any error occurs, it returns silently.
func trim(file *os.File) {
	filestat, err := file.Stat()
	if err != nil {
		return
	}
	if filestat.Size() < loglimit {
		return
	}
	contents := make([]byte, filestat.Size())
	nbytes, err := file.ReadAt(contents, 0)
	if err != nil {
		return
	}
	file.Truncate(0)
	file.Write(contents[nbytes-loglimit : nbytes])
}

// Init initialises the log file and logger.
func Init(ver string) error {
	cachepath, err := logpath(true)
	if err != nil {
		return err
	}
	logpath := path.Join(cachepath, "gin.log")
	logfile, err = os.OpenFile(logpath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("Error creating file %s", logpath)
	}

	flags := log.Ldate | log.Ltime | log.LUTC
	logger = log.New(logfile, "", flags)

	Write("=== LOGINIT ===")
	Write("VERSION: %s", ver)

	return nil
}

// logpath returns the path where gin cache files (logs) should be stored.
func logpath(create bool) (string, error) {
	var err error
	logpath := os.Getenv("GIN_LOG_DIR")
	if logpath == "" {
		logpath = configDirs.QueryCacheFolder().Path
	}
	if create {
		err = os.MkdirAll(logpath, 0755)
		if err != nil {
			return "", fmt.Errorf("could not create log directory %s", logpath)
		}
	}
	return logpath, err
}

// Write writes a string to the log file. Nothing happens if the log file is not initialised (see LogInit).
// Depending on the number of arguments passed, Write either behaves as a Print or a Printf. The first argument must always be a string. If more than one argument is given, the function behaves as Printf.
func Write(fmtstr string, args ...interface{}) {
	if logger == nil {
		return
	}
	logger.Printf(fmtstr, args...)
}

// WriteError prints err to the logfile and returns, effectively ignoring the error.
// No logging is performed if err == nil.
func WriteError(err error) {
	if err != nil {
		Write("The following error occured:\n%s", err)
	}
}

// Close closes the log file.
func Close() {
	Write("=== LOGEND ===")
	trim(logfile)
	_ = logfile.Close()
}
