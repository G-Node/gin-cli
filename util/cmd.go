package util

import (
	"bufio"
	"os/exec"
)

// GinCmd extends the exec.Cmd struct with convenience functions for reading piped output.
// It also overrides the Wait() method such that the error is stored in the Err field.
type GinCmd struct {
	*exec.Cmd
	OutReader *bufio.Reader
	ErrReader *bufio.Reader
	Err       error
}

// Command returns the GinCmd struct to execute the named program with the given arguments.
func Command(name string, args ...string) GinCmd {
	cmd := exec.Command(name, args...)
	outpipe, _ := cmd.StdoutPipe()
	errpipe, _ := cmd.StderrPipe()
	outreader := bufio.NewReader(outpipe)
	errreader := bufio.NewReader(errpipe)
	return GinCmd{cmd, outreader, errreader, nil}
}
