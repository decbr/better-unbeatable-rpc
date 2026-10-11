//go:build !linux

package main

import (
	"context"
	"errors"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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

const clipboardHelperFlag = "--copy-to-clipboard"

func copyText(ctx context.Context, text string) error { return runtime.ClipboardSetText(ctx, text) }

func clipboardHelper() int { return 1 }
