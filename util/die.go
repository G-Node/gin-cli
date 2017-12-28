package util

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

var red = color.New(color.FgRed).SprintFunc()

//Die prints a message to stderr and exits the program with status 1.
func Die(msg string) {
	// fmt.Fprintf(color.Error, "%s %s\n", red("ERROR"), msg)
	// Swap the line above for the line below when (if) https://github.com/fatih/color/pull/87 gets merged
	fmt.Fprintf(os.Stderr, "%s %s\n", "ERROR", msg)
	os.Exit(1)
}

// CheckError exits the program if an error is passed to the function.
// The error message is checked for known error messages and an informative message is printed.
// Otherwise, the error message is printed to stderr.
func CheckError(err error) {
	if err != nil {
		LogWrite(err.Error())
		if strings.Contains(err.Error(), "Error loading user token") {
			Die("This operation requires login.")
		}
		Die(err.Error())
	}
}

// CheckErrorMsg exits the program if an error is passed to the function.
// Before exiting, the given msg string is printed to stderr.
func CheckErrorMsg(err error, msg string) {
	if err != nil {
		LogWrite("The following error occured:\n%sExiting with message: %s", err.Error(), msg)
		Die(msg)
	}
}

// LogError prints err to the logfile and returns, effectively ignoring the error.
// No logging is performed if err == nil.
func LogError(err error) {
	if err != nil {
		LogWrite("The following error occured:\n%s", err.Error())
	}
}

// Error is used to return errors caused by web requests, API calls, or system calls in the web package.
// It implements the error built-in interface. The Error() method returns the Description unless it is not set, in which case it returns the Underlying Error (UError) message.
type Error struct {
	// The error that was returned by the underlying system or API call
	UError string
	// The function where the error originated
	Origin string
	// Human-readable description of error and conditions
	Description string
}

func (e Error) Error() string {
	if e.Description != "" {
		return e.Description
	}
	return e.UError
}
