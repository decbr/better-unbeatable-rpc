//go:build !windows

package main

// defaultLogFile is empty, so the log goes to stderr.
func defaultLogFile() string { return "" }
