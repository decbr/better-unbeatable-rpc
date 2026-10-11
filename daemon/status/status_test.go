package status

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/decbr/better-unbeatable-rpc/daemon/config"
	"github.com/decbr/better-unbeatable-rpc/daemon/internal/discordipc"
	"github.com/decbr/better-unbeatable-rpc/daemon/presence"
)

func TestFetchRoundTrip(t *testing.T) {
	want := Response{
		Version:    "1.2.3",
		ConfigPath: "/x/config.json",
		Config:     config.Config{Style: "spotify", ClientID: "1"},
		Overridden: []string{config.KeyStyle},
		Status: presence.Status{
			GameRunning: true,
			Discord:     presence.DiscordStatus{Connected: true, Socket: "/run/discord-ipc-0"},
			Activity:    &discordipc.Activity{Details: "RE-TUNED", State: "peak divide"},
		},
	}
	srv := httptest.NewServer(Handler(func() Response { return want }))
	defer srv.Close()
	_, portStr, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)

	got, err := Fetch(context.Background(), port)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != want.Version || !reflect.DeepEqual(got.Config, want.Config) || len(got.Overridden) != 1 ||
		!got.GameRunning || got.Discord != want.Discord || got.Activity == nil || *got.Activity != *want.Activity {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestHandlerIsReadOnly(t *testing.T) {
	h := Handler(func() Response { return Response{} })
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/status", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /status = %d, want 405", method, rec.Code)
		}
	}
}

func TestFetchNoDaemon(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	if _, err := Fetch(context.Background(), port); err == nil {
		t.Error("Fetch succeeded with nothing listening")
	}
}
