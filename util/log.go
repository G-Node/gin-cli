package util

import (
	"fmt"
	"log"
	"os"
	"path"
)

// LogFile embeds log.Logger for writing out to a log file and also provides other convenience functions.
type LogFile struct {
	File *os.File
	*log.Logger
}

// LogInit initialises the log file.
func LogInit() (*LogFile, error) {
	dataPath, err := DataPath(true)
	if err != nil {
		return nil, err
	}

	// TODO: Log rotation
	fullPath := path.Join(dataPath, "gin.log")
	file, err := os.OpenFile(fullPath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("Error creating file %s", fullPath)
	}

	flags := log.Ldate | log.Ltime | log.LUTC
	logger := log.New(file, "", flags)

	lf := LogFile{
		File:   file,
		Logger: logger,
	}
	return &lf, nil
}

// Close the log file.
func (lf *LogFile) Close() error {
	return lf.File.Close()
}
