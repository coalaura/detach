package main

import "syscall"

const (
	createBreakawayFromJob = 0x01000000
	createNoWindow         = 0x08000000
)

func processAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: createBreakawayFromJob | createNoWindow,
	}
}
