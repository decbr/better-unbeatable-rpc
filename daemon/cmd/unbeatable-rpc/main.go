package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/decbr/better-unbeatable-rpc/daemon/config"
	"github.com/decbr/better-unbeatable-rpc/daemon/internal/discordipc"
	"github.com/decbr/better-unbeatable-rpc/daemon/presence"
	"github.com/decbr/better-unbeatable-rpc/daemon/status"
)

// version is set by the Makefile.
var version = "dev"

const (
	defaultPort = 47820
	watchEvery  = 2 * time.Second
	largeText   = "UNBEATABLE"
)

func main() {
	defaultPath, _ := config.Path()

	port := flag.Int("port", envInt("UNBEATABLE_RPC_PORT", defaultPort), "UDP port to listen on, 127.0.0.1 only; must match the plugin's UdpPort (env UNBEATABLE_RPC_PORT)")
	statusPort := flag.Int("status-port", envInt(status.PortEnv, status.DefaultPort), "port for the read-only status endpoint on 127.0.0.1; 0 turns it off (env "+status.PortEnv+")")
	configPath := flag.String("config", defaultPath, "settings file")
	logFile := flag.String("log-file", defaultLogFile(), `file to log to, the previous run's kept as .old; "" logs to stderr`)

	// These can also be set in the settings file. Only flags actually given on
	// the command line override it, so their defaults here are empty.
	flagKeys := map[string]string{}
	help := func(key, text string) string {
		return text + " (env " + config.EnvVars[key] + "; overrides the settings file)"
	}
	stringFlag := func(name, key, text string) {
		flag.String(name, "", help(key, text))
		flagKeys[name] = key
	}
	stringFlag("style", config.KeyStyle, "presence text style: "+strings.Join(presence.StyleNames(), ", ")+" or a custom style's name; default "+config.DefaultStyle)
	stringFlag("client-id", config.KeyClientID, "Discord application ID; default "+config.DefaultClientID)
	stringFlag("large-image", config.KeyLargeImage, `art asset key for the large image; default "logo", or "drp_icon" with the game's own client ID`)
	stringFlag("socket", config.KeySocket, "explicit discord-ipc socket path; default is to search")
	flag.Bool("verbose", false, help(config.KeyVerbose, "log every message and activity"))
	flagKeys["verbose"] = config.KeyVerbose
	flag.Parse()

	logger := log.New(os.Stderr, "unbeatable-rpc: ", log.LstdFlags)

	conn, listenErr := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: *port})
	if *logFile != "" {
		if f, err := openLog(*logFile, listenErr == nil); err != nil {
			logger.Printf("log file: %v", err)
		} else {
			defer f.Close()
			logger.SetOutput(f)
		}
	}
	if listenErr != nil {
		logger.Fatalf("listen on 127.0.0.1:%d: %v", *port, listenErr)
	}

	overrides := config.EnvOverrides(os.Getenv)
	flag.Visit(func(f *flag.Flag) {
		if key, ok := flagKeys[f.Name]; ok {
			overrides[key] = f.Value.String()
		}
	})

	d := &daemon{path: *configPath, overrides: overrides, log: logger}
	u, err := d.load(true)
	if err != nil {
		logger.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg := d.current()
	logger.Printf("listening on %s, client ID %s, style %s", conn.LocalAddr(), cfg.ClientID, cfg.Style)
	logger.Printf("settings file: %s", d.path)

	states := make(chan presence.State, 64)
	go receive(ctx, conn, states, logger, &d.verbose)

	m := &presence.Manager{
		Options: u.Options,
		Log:     logger,
		Verbose: u.Verbose,
		Hidden:  u.Hidden,
		Dial: func(ctx context.Context) (*discordipc.Client, error) {
			cfg := d.current()
			var paths []string
			if cfg.Socket != "" {
				paths = []string{cfg.Socket}
			}
			return discordipc.Dial(ctx, cfg.ClientID, paths...)
		},
	}

	go config.Watch(ctx, d.path, watchEvery, func() {
		if u, err := d.load(false); err == nil {
			m.Apply(u)
		}
	})

	if *statusPort != 0 {
		go serveStatus(ctx, *statusPort, logger, func() status.Response { return d.response(m.Status()) })
	}

	m.Run(ctx, states)
	logger.Printf("stopped")
}

// daemon holds the settings currently in force.
type daemon struct {
	path      string
	overrides config.Overrides
	log       *log.Logger
	verbose   atomic.Bool

	mu      sync.Mutex
	cfg     config.Config
	fileErr string
}

// load reads the settings file and works out the settings to use.
func (d *daemon) load(initial bool) (presence.Update, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	prev := d.cfg

	file, fileErr := config.Read(d.path)
	if fileErr != nil && !initial {
		d.fileErr = fileErr.Error()
		d.log.Printf("settings file not reloaded: %v", fileErr)
		return presence.Update{}, fileErr
	}
	cfg, err := config.Resolve(file, d.overrides)
	if err != nil {
		return presence.Update{}, err
	}

	style, styleErr := presence.LookupStyle(cfg.Style, cfg.CustomStyles)
	if styleErr != nil {
		if _, fromOverride := d.overrides[config.KeyStyle]; fromOverride {
			return presence.Update{}, styleErr
		}
		// the previous style as it was, even if the file has since broken or
		// deleted it
		fallback, fallbackCustom := config.DefaultStyle, []presence.CustomStyle(nil)
		if !initial {
			fallback, fallbackCustom = prev.Style, prev.CustomStyles
		}
		if style, err = presence.LookupStyle(fallback, fallbackCustom); err != nil {
			fallback = config.DefaultStyle
			style, _ = presence.LookupStyle(fallback, nil)
		}
		d.log.Printf("settings file: %v; using %s", styleErr, fallback)
		cfg.Style = fallback
	}

	d.fileErr = ""
	switch {
	case fileErr != nil:
		d.fileErr = fileErr.Error()
		d.log.Printf("ignoring settings file: %v", fileErr)
	case styleErr != nil:
		d.fileErr = styleErr.Error()
	}

	d.cfg = cfg
	d.verbose.Store(cfg.Verbose)
	if !initial {
		d.log.Printf("settings reloaded: style %s, client ID %s", cfg.Style, cfg.ClientID)
	}
	return presence.Update{
		Options:   presence.Options{LargeImage: cfg.EffectiveLargeImage(), LargeText: largeText, Style: style},
		Verbose:   cfg.Verbose,
		Hidden:    cfg.Hidden,
		Reconnect: !initial && (cfg.ClientID != prev.ClientID || cfg.Socket != prev.Socket),
	}, nil
}

func (d *daemon) current() config.Config {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cfg
}

func (d *daemon) response(s presence.Status) status.Response {
	d.mu.Lock()
	defer d.mu.Unlock()
	return status.Response{
		Version:     version,
		ConfigPath:  d.path,
		Config:      d.cfg,
		Overridden:  d.overrides.Keys(),
		ConfigError: d.fileErr,
		Status:      s,
	}
}

func serveStatus(ctx context.Context, port int, logger *log.Logger, get func() status.Response) {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		logger.Printf("status endpoint off: %v", err)
		return
	}
	srv := &http.Server{Handler: status.Handler(get), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Printf("status endpoint: %v", err)
	}
}

func receive(ctx context.Context, conn *net.UDPConn, out chan<- presence.State, logger *log.Logger, verbose *atomic.Bool) {
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	buf := make([]byte, 64*1024)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				logger.Printf("udp read: %v", err)
			}
			return
		}
		if verbose.Load() {
			logger.Printf("received %s", buf[:n])
		}
		s, err := presence.ParseState(buf[:n])
		if err != nil {
			logger.Printf("ignoring message: %v", err)
			continue
		}
		select {
		case out <- s:
		case <-ctx.Done():
			return
		}
	}
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unbeatable-rpc: ignoring %s=%q: not a number\n", key, v)
		return def
	}
	return n
}

// openLog opens the log for appending, first moving the last one to .old if rotate is set.
func openLog(path string, rotate bool) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if rotate {
		_ = os.Rename(path, path+".old")
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}
