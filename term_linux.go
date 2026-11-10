//go:build linux

package main

import (
	"syscall"
	"unsafe"
)

type winsize struct {
	Rows   uint16
	Cols   uint16
	Xpixel uint16
	Ypixel uint16
}

func termSizePlatform(fd int) (int, int, bool) {
	ws := &winsize{}
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(ws)))
	if errno != 0 {
		return 0, 0, false
	}
	return int(ws.Rows), int(ws.Cols), true
}
