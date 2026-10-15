package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// init borrows the terminal it was run from, if any, since the build has no console.
func init() {
	if t, err := windows.GetFileType(windows.Handle(os.Stderr.Fd())); err == nil && t != windows.FILE_TYPE_UNKNOWN {
		return
	}
	const attachParentProcess = ^uintptr(0)
	attach := windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")
	if ok, _, _ := attach.Call(attachParentProcess); ok == 0 {
		return
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = f, f
	}
}

// defaultLogFile is where the settings app reads the log from.
func defaultLogFile() string {
	dir, err := os.UserCacheDir() // %LOCALAPPDATA%
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "unbeatable-rpc", "daemon.log")
}
