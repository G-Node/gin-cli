package util

import (
	"bufio"
	"bytes"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

// Reader extends bufio.Reader with convenience functions for reading and caching piped output.
type Reader struct {
	*bufio.Scanner
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
	outreader := bufio.NewScanner(outpipe)
	errreader := bufio.NewScanner(errpipe)
	outreader.Split(scanLinesCR)
	errreader.Split(scanLinesCR)
	var cout, cerr string
	return GinCmd{cmd, &Reader{outreader, cout}, &Reader{errreader, cerr}}
}

// scanLinesCR is a modification of the default split function for the Scanner.
// Instead of splitting just on new line (\n), it also splits on carriage return (\r).
// Unlike most split functions, the delimiter is included in the returned data.
func scanLinesCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	cridx := bytes.IndexByte(data, '\r')
	nlidx := bytes.IndexByte(data, '\n')
	idx := -1
	if cridx >= 0 {
		idx = cridx
	}
	if nlidx >= 0 && nlidx < cridx {
		idx = nlidx
	}
	if idx >= 0 {
		return idx + 1, data[0 : idx+1], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// ReadLine returns the next line in the buffer, delimited by '\n'.
func (reader *Reader) ReadLine() (str string, err error) {
	ok := reader.Scan()
	if ok {
		str = reader.Text()
	} else if err != nil {
		err = reader.Err()
	} else {
		err = io.EOF
	}

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

// CleanSpaces replaces multiple occurences of the space character with one and trims leading and trailing spaces from a string.
func CleanSpaces(str string) string {
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(strings.TrimSpace(str), " ")
}
