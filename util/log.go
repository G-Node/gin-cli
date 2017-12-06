package util

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"
)

var logfile *os.File
var logger *log.Logger

// LogInit initialises the log file and logger.
func LogInit() error {
	// move old log and clear old log path
	oldpath, _ := OldDataPath(false)

	cachepath, err := CachePath(true)
	if err != nil {
		return err
	}

	if _, operr := os.Stat(oldpath); !os.IsNotExist(operr) {
		isodate := time.Now().Format("2006-01-02")
		movedest := path.Join(cachepath, fmt.Sprintf("old-logs-%s", isodate))
		os.Rename(oldpath, movedest)
	}

	// TODO: Log rotation
	fullPath := path.Join(cachepath, "gin.log")
	logfile, err = os.OpenFile(fullPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("Error creating file %s", fullPath)
	}

	flags := log.Ldate | log.Ltime | log.LUTC
	logger = log.New(logfile, "", flags)

	LogWrite("---")

	return nil
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
	_ = logfile.Close()
}
