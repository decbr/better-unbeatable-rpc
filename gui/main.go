package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:            "UNBEATABLE RPC",
		Width:            520,
		Height:           760,
		MinWidth:         420,
		MinHeight:        500,
		AssetServer:      &assetserver.Options{Assets: frontendFS()},
		BackgroundColour: &options.RGBA{R: 30, G: 31, B: 34, A: 255},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		Linux:            &linux.Options{ProgramName: "unbeatable-rpc-settings"},
	})
	if err != nil {
		log.Fatal(err)
	}
}
