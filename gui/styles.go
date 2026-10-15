package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/decbr/better-unbeatable-rpc/daemon/presence"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// A shared style is a custom style with a version marker, so a paste can be
// recognised as one, and an older app can tell a newer style apart.
const styleVersion = 1

type sharedStyle struct {
	Version int `json:"unbeatableRpcStyle"`
	presence.CustomStyle
}

var errNotStyle = errors.New("that isn't an UNBEATABLE RPC style")

func exportStyle(c presence.CustomStyle) string {
	data, _ := json.MarshalIndent(sharedStyle{styleVersion, c}, "", "  ")
	return string(data)
}

// importStyle reads a shared style. Pasted from a chat it may be in a code
// block or have words around it, so it reads from the first { to the last }.
func importStyle(text string) (presence.CustomStyle, error) {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return presence.CustomStyle{}, errNotStyle
	}
	var s sharedStyle
	if err := json.Unmarshal([]byte(text[start:end+1]), &s); err != nil {
		return presence.CustomStyle{}, errNotStyle
	}
	switch {
	case s.Version == 0:
		return presence.CustomStyle{}, errNotStyle
	case s.Version > styleVersion:
		return presence.CustomStyle{}, errors.New("that style is from a newer version of UNBEATABLE RPC: update to import it")
	}
	if _, err := s.Compile(); err != nil {
		return presence.CustomStyle{}, fmt.Errorf("that style doesn't work: %w", err)
	}
	s.Name = strings.TrimSpace(s.Name)
	return s.CustomStyle, nil
}

func (a *App) ExportStyle(c presence.CustomStyle) string { return exportStyle(c) }

func (a *App) ImportStyle(text string) (presence.CustomStyle, error) { return importStyle(text) }

// SaveStyleFile asks where to save the style. It returns false if the user cancelled.
func (a *App) SaveStyleFile(c presence.CustomStyle) (bool, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save style",
		DefaultFilename: styleFileName(c.Name),
		Filters:         []runtime.FileFilter{{DisplayName: "Styles (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return false, err
	}
	return true, os.WriteFile(path, []byte(exportStyle(c)+"\n"), 0o644)
}

// OpenStyleFile asks for a style file and reads it. It returns nil if the
// user cancelled.
func (a *App) OpenStyleFile() (*presence.CustomStyle, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import style",
		Filters: []runtime.FileFilter{{DisplayName: "Styles (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > 1<<20 {
		return nil, errNotStyle
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c, err := importStyle(string(data))
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func styleFileName(name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < ' ' {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		name = "style"
	}
	return name + ".json"
}
