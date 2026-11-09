//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const (
	createBreakawayFromJob = 0x01000000
	createNoWindow         = 0x08000000
)

var version = "dev"

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: detach <executable> [arguments...]")
	}

	if os.Args[1] == "version" {
		fmt.Printf("detach version %s\n", version)

		return nil
	}

	logFile, err := os.CreateTemp("", "detach-*.log")
	if err != nil {
		return fmt.Errorf("create log: %w", err)
	}

	defer logFile.Close()

	nullInput, err := os.Open(os.DevNull)
	if err != nil {
		return fmt.Errorf("open null input: %w", err)
	}

	defer nullInput.Close()

	cmd := exec.Command(os.Args[1], os.Args[2:]...)

	cmd.Stdin = nullInput
	cmd.Stdout = nullInput
	cmd.Stderr = logFile

	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createBreakawayFromJob | createNoWindow,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start process: %w (log: %s)", err, logFile.Name())
	}

	fmt.Printf("PID %d\n", cmd.Process.Pid)
	fmt.Printf("Err: %s\n", logFile.Name())

	err = cmd.Process.Release()
	if err != nil {
		return fmt.Errorf("release process handle: %w", err)
	}

	return nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "detach:", err)

		os.Exit(1)
	}
}
