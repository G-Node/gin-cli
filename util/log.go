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
func LogInit() error {
	dataPath, err := DataPath(true)
	if err != nil {
		return err
	}

	// TODO: Log rotation
	fullPath := path.Join(dataPath, "gin.log")
	logfile, err = os.OpenFile(fullPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("Error creating file %s", fullPath)
	}

	flags := log.Ldate | log.Ltime | log.LUTC
	logger = log.New(logfile, "", flags)

	return nil
}

// LogWrite writes a string to the log file. Nothing happens if the log file is not initialised (see LogInit).
func LogWrite(text string) {
	if logger != nil {
		logger.Print(text)

	}
}

// LogWriteLine writes a string to the log file and terminates with new line. Nothing happens if the log file is not initialised (see LogInit).
func LogWriteLine(text string) {
	if logger != nil {
		logger.Println(text)
	}
}

// LogClose closes the log file.
func LogClose() error {
	return logfile.Close()
}
