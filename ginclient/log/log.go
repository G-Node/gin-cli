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

// Init initialises the log file and logger.
func Init(ver string) error {
	// TODO: Log rotation
	cachepath, err := CachePath(true)
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

// CachePath returns the path where gin cache files (logs) should be stored.
func CachePath(create bool) (string, error) {
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
	if len(args) == 0 {
		logger.Print(fmtstr)
	} else {
		logger.Printf(fmtstr, args...)
	}
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
	_ = logfile.Close()
}
