package main

import "syscall"

func processAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		// A new session has no controlling terminal and is outside the SSH session's process group.
		Setsid: true,
	}
}
