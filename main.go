package main

import (
	"embed"
	"log"

	"github.com/daniel-sabin/pigeon/internal/storage"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dir, err := storage.DefaultDataDir()
	if err != nil {
		log.Fatal(err)
	}
	store, err := storage.New(dir)
	if err != nil {
		log.Fatal(err)
	}
	app := NewApp(store)

	// Without an Edit menu, copy/paste shortcuts don't work in the macOS webview.
	appMenu := menu.NewMenu()
	appMenu.Append(menu.AppMenu())
	appMenu.Append(menu.EditMenu())
	appMenu.Append(menu.WindowMenu())

	err = wails.Run(&options.App{
		Title:     "Pigeon",
		Width:     1280,
		Height:    820,
		MinWidth:  900,
		MinHeight: 560,
		Menu:      appMenu,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 25, B: 29, A: 1},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		Mac: &mac.Options{
			TitleBar:   mac.TitleBarHiddenInset(),
			Appearance: mac.NSAppearanceNameDarkAqua,
			About: &mac.AboutInfo{
				Title:   "Pigeon",
				Message: "A lightweight HTTP client for macOS.",
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
