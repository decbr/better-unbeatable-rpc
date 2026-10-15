package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatestVersion(t *testing.T) {
	for _, tc := range []struct {
		name, version, body string
		status              int
		want                string
	}{
		{"newer", "0.3.1", `{"tag_name":"v0.4.0"}`, 200, "0.4.0"},
		{"same", "0.3.1", `{"tag_name":"v0.3.1"}`, 200, ""},
		{"server error", "0.3.1", `{"tag_name":"v0.4.0"}`, 500, ""},
		{"dev build", "dev", `{"tag_name":"v0.4.0"}`, 200, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			oldURL, oldVersion := latestReleaseURL, version
			defer func() { latestReleaseURL, version = oldURL, oldVersion }()
			latestReleaseURL, version = srv.URL, tc.version

			if got := NewApp().LatestVersion(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
