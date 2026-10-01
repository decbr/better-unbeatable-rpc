package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/decbr/better-unbeatable-rpc/daemon/internal/discordipc"
	"github.com/decbr/better-unbeatable-rpc/daemon/internal/presence"
)

const (
	DefaultClientID = "1555205185792516146"

	GameClientID = "892184493899804682"

	defaultPort = 47820
)

func main() {
	clientID := flag.String("client-id", envOr("UNBEATABLE_RPC_CLIENT_ID", DefaultClientID), "Discord application ID (env UNBEATABLE_RPC_CLIENT_ID)")
	port := flag.Int("port", envInt("UNBEATABLE_RPC_PORT", defaultPort), "UDP port to listen on, 127.0.0.1 only (env UNBEATABLE_RPC_PORT)")
	largeImage := flag.String("large-image", os.Getenv("UNBEATABLE_RPC_LARGE_IMAGE"), `art asset key for the large image; default "logo", or "drp_icon" with the game's own client ID (env UNBEATABLE_RPC_LARGE_IMAGE)`)
	socket := flag.String("socket", os.Getenv("UNBEATABLE_RPC_SOCKET"), "explicit discord-ipc socket path; default is to search (env UNBEATABLE_RPC_SOCKET)")
	verbose := flag.Bool("verbose", envBool("UNBEATABLE_RPC_VERBOSE"), "log every message and activity (env UNBEATABLE_RPC_VERBOSE)")
	flag.Parse()

	logger := log.New(os.Stderr, "unbeatable-rpc: ", log.LstdFlags)

	if *largeImage == "" {
		*largeImage = "logo"
		if *clientID == GameClientID {
			*largeImage = "drp_icon"
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: *port})
	if err != nil {
		logger.Fatalf("listen on 127.0.0.1:%d: %v", *port, err)
	}
	logger.Printf("listening on %s, client ID %s", conn.LocalAddr(), *clientID)

	states := make(chan presence.State, 64)
	go receive(ctx, conn, states, logger, *verbose)

	var paths []string
	if *socket != "" {
		paths = []string{*socket}
	}
	m := &presence.Manager{
		Options: presence.Options{LargeImage: *largeImage, LargeText: "UNBEATABLE"},
		Log:     logger,
		Verbose: *verbose,
		Dial: func(ctx context.Context) (*discordipc.Client, error) {
			return discordipc.Dial(ctx, *clientID, paths...)
		},
	}
	m.Run(ctx, states)
	logger.Printf("stopped")
}

func receive(ctx context.Context, conn *net.UDPConn, out chan<- presence.State, logger *log.Logger, verbose bool) {
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
		if verbose {
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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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

func envBool(key string) bool {
	b, _ := strconv.ParseBool(os.Getenv(key))
	return b
}
