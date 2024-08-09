package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"syscall"
	"time"
)

const KillGrace = 5 * time.Second

func WaitTimeout(c *exec.Cmd, timeout time.Duration) error {
	var kill *time.Timer
	term := time.AfterFunc(timeout, func() {
		err := syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
		if err != nil {
			log.Printf("E! [agent] Error terminating process children: %s", err)
		}
		err = c.Process.Signal(syscall.SIGTERM)
		if err != nil {
			log.Printf("E! [agent] Error terminating process: %s", err)
			return
		}

		kill = time.AfterFunc(KillGrace, func() {
			err := syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
			if err != nil {
				log.Printf("E! [agent] Error terminating process children: %s", err)
			}
			err = c.Process.Kill()
			if err != nil {
				log.Printf("E! [agent] Error killing process: %s", err)
				return
			}
		})
	})

	err := c.Wait()

	// Shutdown all timers
	if kill != nil {
		kill.Stop()
	}
	termSent := !term.Stop()

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
		return nil, fmt.Errorf("looking up lsblk failed: %w", err)
	}
	if path == "" {
		return nil, fmt.Errorf("no path specified for lsblk")
	}
	// path := binName
	cmd := exec.Command(path, arg...)
	out, err := StdOutputTimeout(cmd, CmdTimeout)
	return out, err
}
