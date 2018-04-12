package util

import (
	"fmt"
	"log"
	"os"
	"path"
)

var logfile *os.File
var logger *log.Logger

// LogInit initialises the log file and logger.
func LogInit(ver string) error {
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

	LogWrite("=== LOGINIT ===")
	LogWrite("VERSION: %s", ver)

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

// LogWrite writes a string to the log file. Nothing happens if the log file is not initialised (see LogInit).
// Depending on the number of arguments passed, LogWrite either behaves as a Print or a Printf. The first argument must always be a string. If more than one argument is given, the function behaves as Printf.
func LogWrite(fmtstr string, args ...interface{}) {
	if logger == nil {
		return
	}
	if len(args) == 0 {
		logger.Print(fmtstr)
	} else {
		logger.Printf(fmtstr, args...)
	}
}

// LogClose closes the log file.
func LogClose() {
	LogWrite("=== LOGEND ===")
	_ = logfile.Close()
}
