//go:build !linux

package main

func termSizePlatform(fd int) (int, int, bool) {
	return 0, 0, false
}
