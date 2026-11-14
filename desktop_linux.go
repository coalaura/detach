package main

import "fmt"

func startDesktop(_ string, _ []string) (launchResult, error) {
	return launchResult{}, fmt.Errorf("--desktop is only supported on Windows")
}

func runDesktopHelper(_ []string) error {
	return fmt.Errorf("desktop helper is only supported on Windows")
}
