package main

import (
	"embed"
	"path/filepath"

	"voxflow/internal/config"
	"voxflow/internal/logger"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"voxflow/internal/window"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is stamped at release time via -ldflags "-X main.version=1.2.3".
var version = "dev"

func main() {
	// A packaged .app has no stdout, so mirror logs to ~/.voxflow/voxflow.log.
	if dir, err := config.GetConfigDir(); err == nil {
		if err := logger.File(filepath.Join(dir, "voxflow.log"), logger.INFO); err != nil {
			logger.Warnf("Could not open log file: %v", err)
		}
	}

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
	fileMenu.AddText("Reset Window Position", nil, func(cd *menu.CallbackData) {
		app.ResetWindowPosition()
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
		Width:             window.MiniModeCollapsedW,
		Height:            window.MiniModeCollapsedH,
		MinWidth:          window.MiniModeCollapsedW,
		MinHeight:         window.MiniModeCollapsedH,
		DisableResize:     true, // Disable native resizing to prevent outline artifacts on transparent windows
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
				Message: "AI-Powered Dictation App\n\nVersion " + version,
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
