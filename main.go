package main

import (
	"embed"
	"os"
	"path/filepath"
	"runtime/debug"

	"voxflow/internal/config"
	"voxflow/internal/events"
	"voxflow/internal/logger"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"
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
		// A Finder-launched app's stderr goes nowhere, so a fatal panic would leave no trace.
		if f, err := os.OpenFile(filepath.Join(dir, "crash.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600); err == nil {
			if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
				logger.Warnf("Could not set crash output: %v", err)
			}
			f.Close()
		}
	}

	app := NewApp()

	appMenu := menu.NewMenu()
	appMenu.Append(menu.AppMenu())

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
	fileMenu.AddSeparator()
	fileMenu.AddText("Close Window", keys.CmdOrCtrl("w"), func(cd *menu.CallbackData) {
		app.ShowMiniMode()
	})
	fileMenu.AddText("Quit VoxFlow", keys.CmdOrCtrl("q"), func(cd *menu.CallbackData) {
		app.Quit()
	})

	appMenu.Append(menu.EditMenu())

	// ⌘H is Hide on macOS, so views use ⌘1/⌘2 and ⌘, like other Mac apps.
	viewMenu := appMenu.AddSubmenu("View")
	viewMenu.AddText("Home", keys.CmdOrCtrl("1"), func(cd *menu.CallbackData) {
		app.showMainWindow()
		runtime.EventsEmit(app.ctx, events.OpenHome, nil)
	})
	viewMenu.AddText("History", keys.CmdOrCtrl("2"), func(cd *menu.CallbackData) {
		app.showMainWindow()
		app.OpenHistoryWindow()
	})
	viewMenu.AddText("Settings…", keys.CmdOrCtrl(","), func(cd *menu.CallbackData) {
		app.showMainWindow()
		app.OpenSettings()
	})

	err := wails.Run(&options.App{
		Title:     "VoxFlow",
		Width:     window.MiniModeCollapsedW,
		Height:    window.MiniModeCollapsedH,
		MinWidth:  window.MiniModeCollapsedW,
		MinHeight: window.MiniModeCollapsedH,
		// Native resizing is switched on for the full window only (window.setChrome).
		DisableResize:     true,
		AlwaysOnTop:       true,
		StartHidden:       false,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup:        app.startup,
		OnDomReady:       app.onDomReady,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
			},
			About: &mac.AboutInfo{
				Title:   "VoxFlow",
				Message: "Voice dictation for every app\n\nVersion " + version,
			},
			WebviewIsTransparent: true,
			WindowIsTranslucent:  false,
		},
		Menu: appMenu,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: singleInstanceID(),
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				app.showMainWindow()
			},
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// A second copy would kill this one's whisper-server through the shared PID file.
// Dev builds get their own lock so they can run next to an installed copy.
func singleInstanceID() string {
	if version == "dev" {
		return "com.wails.voxflow.dev"
	}
	return "com.wails.voxflow"
}
