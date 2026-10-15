package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	daemonExe = "unbeatable-rpc.exe"
	gameExe   = "UNBEATABLE.exe"
	runKey    = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue  = "unbeatable-rpc"
)

const (
	serviceText          = " in the background"
	startsAtLoginText    = ", starting at login"
	noServiceText        = "."
	noServiceStoppedText = "Not running, and not installed next to this app."
	uninstallAllText     = "It also removes BepInEx, if the installer added it."
	uninstalledText      = ""
	uninstallerAfterExit = true
	installerName        = "install.cmd"
)

type ServiceInfo struct {
	Installed bool `json:"installed"`
	Active    bool `json:"active"`
	Enabled   bool `json:"enabled"`
}

type platformState struct{}

// BepInEx's winhttp.dll loads natively; the BepInEx file check covers it.
func (a *App) platformChecks(string) []Check { return nil }

func platformDetails() string {
	v := windows.RtlGetVersion()
	return fmt.Sprintf("Windows %d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}

// stateDir holds install.json and daemon.log.
func stateDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "unbeatable-rpc")
}

func daemonPath() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(self), daemonExe), nil
}

func serviceInfo() ServiceInfo {
	var s ServiceInfo
	if path, err := daemonPath(); err == nil {
		_, err := os.Stat(path)
		s.Installed = err == nil
	}
	s.Active = len(processes(daemonExe)) > 0
	if k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE); err == nil {
		_, _, err := k.GetStringValue(runValue)
		s.Enabled = err == nil
		k.Close()
	}
	return s
}

func startDaemon() error {
	if len(processes(daemonExe)) > 0 {
		return nil
	}
	path, err := daemonPath()
	if err != nil {
		return err
	}
	cmd := exec.Command(path)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("couldn't start the daemon: %w", err)
	}
	return cmd.Process.Release()
}

func restartDaemon() error {
	if err := stopProcesses(daemonExe); err != nil {
		return err
	}
	return startDaemon()
}

func setStartAtLogin(on bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !on {
		if err := k.DeleteValue(runValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
		return nil
	}
	path, err := daemonPath()
	if err != nil {
		return err
	}
	return k.SetStringValue(runValue, `"`+path+`"`)
}

func gameProcessRunning() bool { return len(processes(gameExe)) > 0 }

// processes are the IDs of running processes with this exe name.
func processes(name string) []uint32 {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	var pids []uint32
	e := windows.ProcessEntry32{}
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), name) {
			pids = append(pids, e.ProcessID)
		}
	}
	return pids
}

// stopProcesses ends every process with this exe name and waits for them to exit.
func stopProcesses(name string) error {
	for _, pid := range processes(name) {
		h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, pid)
		if err != nil {
			return fmt.Errorf("couldn't stop %s: %w", name, err)
		}
		err = windows.TerminateProcess(h, 1)
		if err == nil {
			_, err = windows.WaitForSingleObject(h, 3000)
		}
		windows.CloseHandle(h)
		if err != nil {
			return fmt.Errorf("couldn't stop %s: %w", name, err)
		}
	}
	return nil
}

// daemonLog is the last n lines of the daemon's log file.
func daemonLog(n int) string {
	data, err := os.ReadFile(filepath.Join(stateDir(), "daemon.log"))
	if err != nil {
		return ""
	}
	return lastLines(strings.ReplaceAll(string(data), "\r\n", "\n"), n)
}

// uninstallerPath is the copy of install.ps1 the installer puts next to this app.
func uninstallerPath() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Dir(self), "install.ps1")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("couldn't find %s; run install.ps1 -Uninstall from the release folder instead", path)
	}
	return path, nil
}

// runUninstaller starts the uninstaller, which waits for this app to quit.
func runUninstaller(path string, all bool) (string, error) {
	args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", path, "-Uninstall", "-WaitPid", strconv.Itoa(os.Getpid())}
	if all {
		args = append(args, "-All")
	}
	cmd := exec.Command("powershell.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	return "", cmd.Process.Release()
}

// installState is install.ps1's install.json.
type installState struct {
	GameDir string `json:"gameDir"`
}

// installedGameDir is the game folder the installer used.
func installedGameDir() string {
	data, err := os.ReadFile(filepath.Join(stateDir(), "install.json"))
	if err != nil {
		return ""
	}
	var s installState
	if json.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &s) != nil {
		return ""
	}
	return s.GameDir
}

func openFolder(path string) error {
	// explorer exits 1 even when it worked, so only starting it is checked
	cmd := exec.Command("explorer.exe", path)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("couldn't open %s: %w", path, err)
	}
	go cmd.Wait()
	return nil
}
