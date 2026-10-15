// Package status is the daemon's read-only status endpoint and its client.
// Settings are changed through the config file, never through this endpoint.
package status

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/decbr/better-unbeatable-rpc/daemon/config"
	"github.com/decbr/better-unbeatable-rpc/daemon/presence"
)

const (
	DefaultPort = 47821
	PortEnv     = "UNBEATABLE_RPC_STATUS_PORT"
)

type Response struct {
	Version     string        `json:"version"`
	ConfigPath  string        `json:"configPath"`
	Config      config.Config `json:"config"`
	Overridden  []string      `json:"overridden"` // Overridden lists the config keys set by environment variables or flags, which the file can't change.
	ConfigError string        `json:"configError,omitempty"`
	presence.Status
}

func Handler(get func() Response) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// a web page could reach this through DNS rebinding, under its own host name
		if host, _, _ := net.SplitHostPort(r.Host); host != "127.0.0.1" && host != "localhost" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(get())
	})
	return mux
}

func Fetch(ctx context.Context, port int) (Response, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/status", nil)
	if err != nil {
		return Response{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("status endpoint: %s", resp.Status)
	}
	var r Response
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return Response{}, err
	}
	return r, nil
}
