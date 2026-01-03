package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application menu
	appMenu := menu.NewMenu()

	// App menu (macOS specific)
	appMenu.Append(menu.AppMenu())

	// File menu
	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Toggle Recording", keys.CmdOrCtrl("r"), func(cd *menu.CallbackData) {
		app.ToggleRecording()
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("View History", keys.CmdOrCtrl("h"), func(cd *menu.CallbackData) {
		app.OpenHistoryWindow()
	})
	fileMenu.AddText("Settings", keys.CmdOrCtrl(","), func(cd *menu.CallbackData) {
		app.OpenSettings()
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Quit voxflow", keys.CmdOrCtrl("q"), func(cd *menu.CallbackData) {
		app.Quit()
	})

	// Edit menu
	appMenu.Append(menu.EditMenu())

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "voxflow",
		Width:     900,
		Height:    600,
		MinWidth:  700,
		MinHeight: 500,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 24, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
			},
			About: &mac.AboutInfo{
				Title:   "voxflow",
				Message: "AI-Powered Dictation App\n\nVersion 1.0.0",
			},
			Appearance: mac.NSAppearanceNameDarkAqua,
		},
		Menu: appMenu,
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
