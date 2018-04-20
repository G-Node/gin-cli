package util

import (
	"github.com/fatih/color"
)

var red = color.New(color.FgRed).SprintFunc()

// Error is used to return errors caused by web requests, API calls, or system calls.
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
