// Package shell augments the standard library os/exec Cmd struct and functions
// with convenience functions for reading piped output.
package shell

import (
	"bufio"
	"bytes"
	"os/exec"
)

// Cmd extends the exec.Cmd struct with convenience functions for reading piped
// output.
type Cmd struct {
	*exec.Cmd
	OutReader *bufio.Reader
	ErrReader *bufio.Reader
	Err       error
}

// Command returns the GinCmd struct to execute the named program with the
// given arguments.
func Command(name string, args ...string) Cmd {
	cmd := exec.Command(name, args...)
	outpipe, _ := cmd.StdoutPipe()
	errpipe, _ := cmd.StderrPipe()
	outreader := bufio.NewReader(outpipe)
	errreader := bufio.NewReader(errpipe)
	return Cmd{cmd, outreader, errreader, nil}
}

// OutputError runs the command and returns the standard output and standard
// error as two byte slices.
func (cmd *Cmd) OutputError() ([]byte, []byte, error) {
	var bout, berr bytes.Buffer
	cmd.Stdout = &bout
	cmd.Stderr = &berr
	err := cmd.Run()
	return bout.Bytes(), berr.Bytes(), err
}

// Output runs the command and returns its standard output.
func (cmd *Cmd) Output() ([]byte, error) {
	cmd.Stdout = nil
	return cmd.Cmd.Output()
}

// Error is used to return errors caused by web requests, API calls, or system
// calls.  It implements the error built-in interface. The Error() method
// returns the Description unless it is not set, in which case it returns the
// Underlying Error (UError) message.
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
