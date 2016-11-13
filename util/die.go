package util

import (
	"fmt"
	"os"
)

// CheckError exits the program if an error is passed to the function.
// The function should be used to avoid constnatly checking if `err != nil` and returning errors up the stack when all that needs to be done is to stop execution.
func CheckError(err error) {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
