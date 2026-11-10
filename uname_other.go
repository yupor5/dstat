//go:build !linux

package main

func unameRelease() string { return "unknown" }
