package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

var latestReleaseURL = "https://api.github.com/repos/decbr/better-unbeatable-rpc/releases/latest"

func (a *App) LatestVersion() string {
	if version == "dev" {
		return ""
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(latestReleaseURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var release struct {
		Tag string `json:"tag_name"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&release) != nil {
		return ""
	}
	if latest := strings.TrimPrefix(release.Tag, "v"); latest != "" && latest != version {
		return latest
	}
	return ""
}
