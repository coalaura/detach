//go:build linux || windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

const usage = "usage: detach [--desktop] <executable> [arguments...]"

type launchResult struct {
	PID int
	Log string
}

var version = "dev"

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("%s", usage)
	}

	if os.Args[1] == "version" {
		fmt.Printf("detach version %s\n", version)

		return nil
	}

	arguments := os.Args[1:]
	if arguments[0] == "--desktop-helper" {
		return runDesktopHelper(arguments[1:])
	}

	desktop := arguments[0] == "--desktop"
	if desktop {
		arguments = arguments[1:]
	}

	if len(arguments) > 0 && arguments[0] == "--" {
		arguments = arguments[1:]
	}

	if len(arguments) == 0 {
		return fmt.Errorf("%s", usage)
	}

	var (
		result launchResult
		err    error
	)

	if desktop {
		result, err = startDesktop(arguments[0], arguments[1:])
	} else {
		result, err = startProcess(arguments[0], arguments[1:])
	}

	if result.PID != 0 {
		fmt.Printf("PID %d\n", result.PID)
		fmt.Printf("Err: %s\n", result.Log)
	}

	return err
}

func startProcess(executable string, arguments []string) (launchResult, error) {
	logFile, err := os.CreateTemp("", "detach-*.log")
	if err != nil {
		return launchResult{}, fmt.Errorf("create log: %w", err)
	}

	defer logFile.Close()

	nullFile, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return launchResult{}, fmt.Errorf("open null device: %w", err)
	}

	defer nullFile.Close()

	cmd := exec.Command(executable, arguments...)

	cmd.Stdin = nullFile
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = processAttributes()

	err = cmd.Start()
	if err != nil {
		return launchResult{}, fmt.Errorf("start process: %w (log: %s)", err, logFile.Name())
	}

	result := launchResult{
		PID: cmd.Process.Pid,
		Log: logFile.Name(),
	}

	err = cmd.Process.Release()
	if err != nil {
		return result, fmt.Errorf("release process handle for PID %d: %w (log: %s)", result.PID, err, result.Log)
	}

	return result, nil
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "detach:", err)

		os.Exit(1)
	}
}
