//go:build !linux

package main

import "errors"

var errUnsupported = errors.New("not supported on this OS yet")

func startDaemon() error { return errUnsupported }

func uninstallerPath() (string, error) { return "", errUnsupported }

func runUninstaller(string, bool) (string, error) { return "", errUnsupported }
