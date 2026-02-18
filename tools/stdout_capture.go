package tools

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
)

const maxBacktestLogBytes = 1 << 20 // 1 MiB

var stdoutCaptureMu sync.Mutex

type capturedLogs struct {
	Lines     []string
	Truncated bool
}

func captureStdoutLines(fn func() error) (captured capturedLogs, err error) {
	stdoutCaptureMu.Lock()
	defer stdoutCaptureMu.Unlock()

	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		return capturedLogs{}, pipeErr
	}

	// Redirect global stdout while the backtest is running.
	os.Stdout = w

	var buf bytes.Buffer
	done := make(chan struct{})
	var truncated bool
	go func() {
		defer close(done)
		defer r.Close()

		tmp := make([]byte, 4096)
		for {
			n, readErr := r.Read(tmp)
			if n > 0 {
				remaining := maxBacktestLogBytes - buf.Len()
				if remaining > 0 {
					if n > remaining {
						_, _ = buf.Write(tmp[:remaining])
						truncated = true
					} else {
						_, _ = buf.Write(tmp[:n])
					}
				} else {
					truncated = true
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	var panicVal interface{}
	funcErr := func() (runErr error) {
		defer func() {
			if r := recover(); r != nil {
				panicVal = r
			}
		}()
		return fn()
	}()

	// Restore stdout and complete capture.
	_ = w.Close()
	os.Stdout = oldStdout
	<-done

	out := buf.String()
	out = strings.ReplaceAll(out, "\r\n", "\n")
	lines := strings.Split(out, "\n")
	// Drop trailing empty line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if panicVal != nil {
		panic(panicVal)
	}

	return capturedLogs{Lines: lines, Truncated: truncated}, funcErr
}

func suppressStdout(fn func() error) (err error) {
	stdoutCaptureMu.Lock()
	defer stdoutCaptureMu.Unlock()

	oldStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		return pipeErr
	}

	// Redirect global stdout while the backtest is running.
	os.Stdout = w

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer r.Close()
		_, _ = io.Copy(io.Discard, r)
	}()

	var panicVal interface{}
	funcErr := func() (runErr error) {
		defer func() {
			if r := recover(); r != nil {
				panicVal = r
			}
		}()
		return fn()
	}()

	// Restore stdout and complete capture.
	_ = w.Close()
	os.Stdout = oldStdout
	<-done

	if panicVal != nil {
		panic(panicVal)
	}

	return funcErr
}
