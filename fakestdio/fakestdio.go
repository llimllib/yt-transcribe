//go:build !windows

package fakestdio

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/sys/unix"
)

// modified from: https://eli.thegreenplace.net/2020/faking-stdin-and-stdout-in-go/
//
// FakeStdio can be used to fake stdin and capture stdout.
// Between creating a new FakeStdio and calling ReadAndRestore on it,
// code reading os.Stdin will get the contents of stdinText passed to New.
// Output to os.Stdout will be captured and returned from ReadAndRestore.
// FakeStdio is not reusable; don't attempt to use it after calling
// ReadAndRestore, but it should be safe to create a new FakeStdio.
type FakeStdOutErr struct {
	origStdout   int
	stdoutReader *os.File
	stdoutCh     chan []byte

	origStderr   int
	stderrReader *os.File
	stderrCh     chan []byte
}

func New() (*FakeStdOutErr, error) {
	// Pipe for stdout.
	//
	//               ======
	//  w ----------->||||------> r
	// (os.Stdout)   ======      (stdoutReader)
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	origStdout, err := unix.Dup(unix.Stdout)
	if err != nil {
		return nil, err
	}

	// Clone the pipe's writer to the actual Stdout descriptor; from this point
	// on, writes to Stdout will go to stdoutWriter.
	if err = unix.Dup2(int(stdoutWriter.Fd()), unix.Stdout); err != nil {
		return nil, err
	}

	stdoutCh := make(chan []byte)

	// This goroutine reads stdout into a buffer in the background.
	go func() {
		var b bytes.Buffer
		if _, err := io.Copy(&b, stdoutReader); err != nil {
			log.Println(err)
		}
		stdoutCh <- b.Bytes()
	}()

	// Pipe for stderr.
	//
	//               ======
	//  w ----------->||||------> r
	// (os.Stderr)   ======      (stderrReader)
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	origStderr, err := unix.Dup(unix.Stderr)
	if err != nil {
		return nil, err
	}

	// Clone the pipe's writer to the actual Stderr descriptor; from this point
	// on, writes to Stderr will go to stderrWriter.
	if err = unix.Dup2(int(stderrWriter.Fd()), unix.Stderr); err != nil {
		return nil, err
	}

	stderrCh := make(chan []byte)

	// This goroutine reads stderr into a buffer in the background.
	go func() {
		var b bytes.Buffer
		if _, err := io.Copy(&b, stderrReader); err != nil {
			log.Println(err)
		}
		stderrCh <- b.Bytes()
	}()

	return &FakeStdOutErr{
		origStdout:   origStdout,
		stdoutReader: stdoutReader,
		stdoutCh:     stdoutCh,

		origStderr:   origStderr,
		stderrReader: stderrReader,
		stderrCh:     stderrCh,
	}, nil
}

// ReadAndRestore collects all captured stdout and returns it; it also restores
// os.Stdin and os.Stdout to their original values.
func (sf *FakeStdOutErr) ReadAndRestore() ([]byte, []byte, error) {
	if sf.stdoutReader == nil || sf.stderrReader == nil {
		return nil, nil, fmt.Errorf("ReadAndRestore from closed FakeStdio")
	}

	// Close and null out our reader pipes
	sf.stdoutReader.Close()
	sf.stdoutReader = nil
	sf.stderrReader.Close()
	sf.stderrReader = nil

	unix.Close(unix.Stdout)
	unix.Close(unix.Stderr)

	// Close the writer side of the faked stdout pipe. This signals to the
	// background goroutine that it should exit.
	stdoutBuf := <-sf.stdoutCh
	stderrBuf := <-sf.stderrCh

	// restore stdout and stderr, and close the dup'ed handles
	unix.Dup2(sf.origStdout, unix.Stdout)
	unix.Close(sf.origStdout)
	unix.Dup2(sf.origStderr, unix.Stderr)
	unix.Close(sf.origStderr)

	return stdoutBuf, stderrBuf, nil
}
