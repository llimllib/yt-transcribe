//go:build windows

// NOTE that this is completely untested... I'm just guessing what might work
// based on code samples so far. I have no idea how to even run tests on windows
//       https://stackoverflow.com/a/34773942/42559
//       https://github.com/moby/moby/blob/5f48fc36e158eb63e4cbc220c5bbc784c35f2ae2/cmd/dockerd/service_windows.go#L369

package fakestdio

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/sys/windows"
)

type FakeStdOutErr struct {
	origStdout   int
	stdoutReader *os.File
	stdoutCh     chan []byte

	origStderr   int
	stderrReader *os.File
	stderrCh     chan []byte
}

func New() (*FakeStdOutErr, error) {
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	origStdout, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		return nil, err
	}

	err = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(stdoutWriter.Fd()))
	if err != nil {
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

	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	origStderr, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE)
	if err != nil {
		return nil, err
	}

	err = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(stderrWriter.Fd()))
	if err != nil {
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

	windows.Close(windows.Stdout)
	windows.Close(windows.Stderr)

	// Close the writer side of the faked stdout pipe. This signals to the
	// background goroutine that it should exit.
	stdoutBuf := <-sf.stdoutCh
	stderrBuf := <-sf.stderrCh

	// restore stdout and stderr
	windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, sf.origStdout)
	windows.SetStdHandle(windows.STD_ERROR_HANDLE, sf.origStderr)

	return stdoutBuf, stderrBuf, nil
}
