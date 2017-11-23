package util

import (
	"bufio"
	"os/exec"
)

// Reader extends bufio.Reader with convenience functions for reading and caching piped output.
type Reader struct {
	*bufio.Reader
	cache string
}

// GinCmd extends the exec.Cmd struct with convenience functions for reading piped output.
type GinCmd struct {
	*exec.Cmd
	OutPipe *Reader
	ErrPipe *Reader
}

// Command returns the GinCmd struct to execute the named program with the given arguments.
func Command(name string, args ...string) GinCmd {
	cmd := exec.Command(name, args...)
	outpipe, _ := cmd.StdoutPipe()
	errpipe, _ := cmd.StderrPipe()
	outreader := bufio.NewReader(outpipe)
	errreader := bufio.NewReader(errpipe)
	var cout, cerr string
	return GinCmd{cmd, &Reader{outreader, cout}, &Reader{errreader, cerr}}
}

// ReadLine returns the next line in the buffer, delimited by '\n'.
func (reader *Reader) ReadLine() (string, error) {
	str, err := reader.ReadString('\n')
	reader.cache += str
	return str, err
}

// ReadAll collects the buffer output until it reaches EOF and returns the entire output as a single string.
// The outptut is cached so repeated calls to ReadAll return the output again.
func (reader *Reader) ReadAll() (output string) {
	for {
		_, err := reader.ReadLine()
		if err != nil {
			break
		}
	}
	return reader.cache
}

// LogStdOutErr writes the command stdout and stderr to the log file.
func (cmd *GinCmd) LogStdOutErr() {
	stdout := cmd.OutPipe.ReadAll()
	stderr := cmd.ErrPipe.ReadAll()
	LogWrite("[stdout]\r\n%s", stdout)
	LogWrite("[stderr]\r\n%s", stderr)
}

// Wait collects stdout and stderr from the command and then waits for the command to finish before returning.
// Stdout and stderr are then available for reading through each pipe's respective cache.
func (cmd *GinCmd) Wait() error {
	_ = cmd.OutPipe.ReadAll()
	_ = cmd.ErrPipe.ReadAll()
	return cmd.Cmd.Wait()
}
