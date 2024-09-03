//go:build windows

package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"time"
)

const CmdTimeout = time.Duration(5 * time.Second)

// WaitTimeout waits for the given command to finish with a timeout.
// It assumes the command has already been started.
// If the command times out, it attempts to kill the process.
func WaitTimeout(c *exec.Cmd, timeout time.Duration) error {
	timer := time.AfterFunc(timeout, func() {
		err := c.Process.Kill()
		if err != nil {
			log.Printf("E! [agent] Error killing process: %s", err)
			return
		}
	})

	err := c.Wait()

	// Shutdown all timers
	termSent := !timer.Stop()

	// If the process exited without error treat it as success.  This allows a
	// process to do a clean shutdown on signal.
	if err == nil {
		return nil
	}

	// If SIGTERM was sent then treat any process error as a timeout.
	if termSent {
		return errors.New("command timed out")
	}

	// Otherwise there was an error unrelated to termination.
	return err
}

func StdOutputTimeout(c *exec.Cmd, timeout time.Duration) ([]byte, error) {
	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = nil
	if err := c.Start(); err != nil {
		return nil, err
	}
	err := WaitTimeout(c, timeout)
	return b.Bytes(), err
}

func Exec(binName string, arg ...string) ([]byte, error) {
	path, err := exec.LookPath(binName)
	if err != nil {
		return nil, fmt.Errorf("looking up %s failed: %w", binName, err)
	}
	if path == "" {
		return nil, fmt.Errorf("no path specified for %s", binName)
	}
	// path := binName
	cmd := exec.Command(path, arg...)
	out, err := StdOutputTimeout(cmd, CmdTimeout)
	return out, err
}
