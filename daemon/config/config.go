// Package config is the daemon's settings file, shared with the settings app.
//
// Precedence, lowest first: defaults, the file, UNBEATABLE_RPC_* environment
// variables, command-line flags. The UDP and status ports are not in the file:
// the UDP port has to match the plugin's config, so it stays a flag.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/decbr/better-unbeatable-rpc/daemon/presence"
)

const (
	// DefaultClientID is the "UNBEATABLE" Discord application.
	DefaultClientID = "1555205185792516146"
	// GameClientID is the game's own leftover "UNBEATABLE [white label]" application.
	GameClientID = "892184493899804682"

	DefaultStyle = presence.DefaultStyle
)

type Config struct {
	Style      string `json:"style,omitempty"`
	LargeImage string `json:"largeImage,omitempty"`
	ClientID   string `json:"clientId,omitempty"`
	Socket     string `json:"socket,omitempty"`
	Verbose    bool   `json:"verbose,omitempty"`
}

// Keys and the environment variable that overrides each one.
const (
	KeyStyle      = "style"
	KeyLargeImage = "largeImage"
	KeyClientID   = "clientId"
	KeySocket     = "socket"
	KeyVerbose    = "verbose"
)

var EnvVars = map[string]string{
	KeyStyle:      "UNBEATABLE_RPC_STYLE",
	KeyLargeImage: "UNBEATABLE_RPC_LARGE_IMAGE",
	KeyClientID:   "UNBEATABLE_RPC_CLIENT_ID",
	KeySocket:     "UNBEATABLE_RPC_SOCKET",
	KeyVerbose:    "UNBEATABLE_RPC_VERBOSE",
}

func Defaults() Config {
	return Config{Style: DefaultStyle, ClientID: DefaultClientID}
}

// EffectiveLargeImage is the art asset to use: the configured one, or the
// right default for the client ID.
func (c Config) EffectiveLargeImage() string {
	switch {
	case c.LargeImage != "":
		return c.LargeImage
	case c.ClientID == GameClientID:
		return "drp_icon"
	default:
		return "logo"
	}
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "unbeatable-rpc", "config.json"), nil
}

// Read reads the file. A missing file is not an error: it gives an empty Config.
func Read(path string) (Config, error) {
	var c Config
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// Save writes c atomically, so the daemon never reads half a file.
func Save(path string, c Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Overrides are values set by environment variables or flags, by key. They
// win over the file.
type Overrides map[string]string

func EnvOverrides(getenv func(string) string) Overrides {
	o := Overrides{}
	for key, env := range EnvVars {
		if v := getenv(env); v != "" {
			o[key] = v
		}
	}
	return o
}

func (o Overrides) Keys() []string {
	keys := make([]string, 0, len(o))
	for k := range o {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Resolve layers the file, then the overrides, on top of the defaults.
func Resolve(file Config, o Overrides) (Config, error) {
	c := Defaults()
	if file.Style != "" {
		c.Style = file.Style
	}
	if file.LargeImage != "" {
		c.LargeImage = file.LargeImage
	}
	if file.ClientID != "" {
		c.ClientID = file.ClientID
	}
	if file.Socket != "" {
		c.Socket = file.Socket
	}
	c.Verbose = file.Verbose

	for key, v := range o {
		switch key {
		case KeyStyle:
			c.Style = v
		case KeyLargeImage:
			c.LargeImage = v
		case KeyClientID:
			c.ClientID = v
		case KeySocket:
			c.Socket = v
		case KeyVerbose:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return c, fmt.Errorf("%s: %q is not true or false", KeyVerbose, v)
			}
			c.Verbose = b
		default:
			return c, fmt.Errorf("unknown setting %q", key)
		}
	}
	return c, nil
}
