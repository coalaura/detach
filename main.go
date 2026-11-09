//go:build linux || windows

package main

import (
	"fmt"
	"os"
	"os/exec"
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

	nullFile, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open null device: %w", err)
	}

	defer nullFile.Close()

	cmd := exec.Command(os.Args[1], os.Args[2:]...)

	cmd.Stdin = nullFile
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = processAttributes()

	err = cmd.Start()
	if err != nil {
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
