//go:build !linux && !windows

package main

import (
	"errors"
	"runtime"
)

var errUnsupported = errors.New("not supported on this OS yet")

func startDaemon() error         { return errUnsupported }
func restartDaemon() error       { return errUnsupported }
func setStartAtLogin(bool) error { return errUnsupported }
func serviceInfo() ServiceInfo   { return ServiceInfo{} }
func gameProcessRunning() bool   { return false }
func daemonLog(int) string       { return "" }

type ServiceInfo struct {
	Installed bool `json:"installed"`
	Active    bool `json:"active"`
	Enabled   bool `json:"enabled"`
}

func uninstallerPath() (string, error) { return "", errUnsupported }

func runUninstaller(string, bool) (string, error) { return "", errUnsupported }

func installedGameDir() string { return "" }

func openFolder(string) error { return errUnsupported }

type platformState struct{}

func (a *App) platformChecks(string) []Check { return nil }

func platformDetails() string { return runtime.GOOS }

const (
	serviceText          = ""
	startsAtLoginText    = ""
	noServiceText        = "."
	noServiceStoppedText = "Not running."
	uninstallAllText     = "It also removes BepInEx, if the installer added it."
	uninstalledText      = ""
	uninstallerAfterExit = false
	installerName        = "the installer"
)
