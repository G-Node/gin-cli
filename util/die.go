package util

import (
	"fmt"
	"os"
)

//Die prints a message to stderr and exits the program.
func Die(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

// CheckError exits the program if an error is passed to the function.
// Before exiting, the error message is printed to stderr.
// The function should be used to avoid constnatly checking if `err != nil` and returning errors up the stack when all that needs to be done is to stop execution.
func CheckError(err error) {
	if err != nil {
		// TODO: Print error to log
		Die(err.Error())
	}
}

// CheckErrorMsg exits the program if an error is passed to the function.
// Before exiting, the given msg string is printed to stderr.
func CheckErrorMsg(err error, msg string) {
	if err != nil {
		// TODO: Print actual error to log
		Die(msg)
	}
}
