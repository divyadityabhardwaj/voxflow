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
	fileMenu.AddText("Open Full App", keys.CmdOrCtrl("o"), func(cd *menu.CallbackData) {
		app.HideMiniMode()
	})
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

	// Create application with options - Start as floating indicator
	err := wails.Run(&options.App{
		Title:             "voxflow",
		Width:             200,
		Height:            60,
		MinWidth:          200,
		MinHeight:         60,
		DisableResize:     true,
		Frameless:         true,
		AlwaysOnTop:       true,
		StartHidden:       false,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0}, // Transparent
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               true,
				FullSizeContent:            true,
				UseToolbar:                 false,
			},
			About: &mac.AboutInfo{
				Title:   "voxflow",
				Message: "AI-Powered Dictation App\n\nVersion 1.0.0",
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
		},
		Menu: appMenu,
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
