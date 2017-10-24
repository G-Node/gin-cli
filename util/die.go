package util

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

var red = color.New(color.FgRed)

//Die prints a message to stderr and exits the program.
func Die(msg string) {
	_, _ = red.Fprint(os.Stderr, "ERROR ")
	fmt.Fprintln(os.Stderr, msg)
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
