package config

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadMissingFileIsEmpty(t *testing.T) {
	c, err := Read(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil || c != (Config{}) {
		t.Errorf("got %+v, %v; want empty config, nil", c, err)
	}
}

func TestReadBadJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte(`{"style":`), 0o644)
	if _, err := Read(path); err == nil {
		t.Error("Read of bad JSON succeeded")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")
	want := Config{Style: "spotify", Socket: "/run/x", Verbose: true}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil || got != want {
		t.Errorf("got %+v, %v; want %+v", got, err, want)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("left %d files behind, want only config.json", len(entries))
	}
}

func TestResolvePrecedence(t *testing.T) {
	file := Config{Style: "spotify", ClientID: "111", Socket: "/file", Verbose: true}
	got, err := Resolve(file, Overrides{KeyClientID: "222", KeyVerbose: "0"})
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Style: "spotify", ClientID: "222", Socket: "/file", Verbose: false}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	if got, _ := Resolve(Config{}, nil); got != Defaults() {
		t.Errorf("empty file: got %+v, want defaults %+v", got, Defaults())
	}
	if _, err := Resolve(Config{}, Overrides{KeyVerbose: "maybe"}); err == nil {
		t.Error("bad verbose override accepted")
	}
}

func TestEnvOverrides(t *testing.T) {
	env := map[string]string{"UNBEATABLE_RPC_STYLE": "spotify", "UNBEATABLE_RPC_SOCKET": "", "OTHER": "x"}
	o := EnvOverrides(func(k string) string { return env[k] })
	if !reflect.DeepEqual(o, Overrides{KeyStyle: "spotify"}) {
		t.Errorf("got %v", o)
	}
	if keys := (Overrides{KeyStyle: "", KeyClientID: ""}).Keys(); !reflect.DeepEqual(keys, []string{KeyClientID, KeyStyle}) {
		t.Errorf("keys %v not sorted", keys)
	}
}

func TestEffectiveLargeImage(t *testing.T) {
	for _, c := range []struct {
		in   Config
		want string
	}{
		{Config{ClientID: DefaultClientID}, "logo"},
		{Config{ClientID: GameClientID}, "drp_icon"},
		{Config{ClientID: GameClientID, LargeImage: "x"}, "x"},
	} {
		if got := c.in.EffectiveLargeImage(); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.in, got, c.want)
		}
	}
}

func TestWatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	var calls atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { Watch(ctx, path, 10*time.Millisecond, func() { calls.Add(1) }); close(done) }()
	t.Cleanup(func() { cancel(); <-done })

	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 0 {
		t.Fatal("called without a change")
	}
	if err := Save(path, Config{Style: "spotify"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d after creating the file, want 1", calls.Load())
	}
}

func TestResolveHidden(t *testing.T) {
	if c, _ := Resolve(Config{Hidden: true}, nil); !c.Hidden {
		t.Error("hidden in the file was ignored")
	}
	if c, _ := Resolve(Config{Hidden: true}, Overrides{KeyHidden: "false"}); c.Hidden {
		t.Error("UNBEATABLE_RPC_HIDDEN=false didn't win over the file")
	}
	if _, err := Resolve(Config{}, Overrides{KeyHidden: "sometimes"}); err == nil {
		t.Error("bad hidden override accepted")
	}
}
