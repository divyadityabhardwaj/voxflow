package main

import (
	"context"
	"time"
	"voxflow/internal/events"
	"voxflow/internal/logger"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ShowMiniMode switches the window to a small floating indicator
func (a *App) ShowMiniMode() {
	if a.isMiniMode {
		return
	}
	a.isMiniMode = true
	a.userExplicitlyMaximized = false // User explicitly minimized

	// Re-apply floating behavior (in case coming from full app mode)
	MakeWindowFloatEverywhere()

	// Set constrained range so the mini window can't grow beyond the expanded strip.
	// Then SetMiniModeExpanded can smoothly tween width within this range.
	runtime.WindowSetMinSize(a.ctx, miniModeCollapsedW, miniModeCollapsedH)
	runtime.WindowSetMaxSize(a.ctx, miniModeExpandedW, miniModeExpandedH)
	runtime.WindowSetSize(a.ctx, miniModeCollapsedW, miniModeCollapsedH)

	// Restore saved position if available
	x, y := a.config.GetMiniModePosition()
	if x != 0 || y != 0 {
		runtime.WindowSetPosition(a.ctx, x, y)
	}

	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.EventsEmit(a.ctx, events.MiniMode, true)

	// Start watching position for changes
	a.startPositionWatch()

	logger.Infof("[App] Switched to mini mode")
}

// HideMiniMode restores the window to normal size
func (a *App) HideMiniMode() {
	if !a.isMiniMode {
		return
	}

	// Stop position watching
	if a.positionWatchCancel != nil {
		a.positionWatchCancel()
		a.positionWatchCancel = nil
	}

	// Save current position one last time
	a.saveCurrentMiniModePosition()

	a.isMiniMode = false
	a.userExplicitlyMaximized = true // User explicitly opened full app

	// Reset window behavior to normal (not floating over fullscreen)
	ResetWindowBehavior()

	// Restore saved maximized position and size if available
	savedX, savedY := a.config.GetMaximizedWindowPosition()
	savedW, savedH := a.config.GetMaximizedWindowSize()

	// Set normal window size limits
	runtime.WindowSetMinSize(a.ctx, 800, 600)
	runtime.WindowSetMaxSize(a.ctx, 0, 0)

	// Restore saved size or use default
	if savedW > 0 && savedH > 0 {
		runtime.WindowSetSize(a.ctx, savedW, savedH)
	} else {
		runtime.WindowSetSize(a.ctx, 900, 600)
	}

	// Restore saved position or center if not saved
	if savedX != 0 || savedY != 0 {
		runtime.WindowSetPosition(a.ctx, savedX, savedY)
	} else {
		runtime.WindowCenter(a.ctx)
	}

	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	runtime.EventsEmit(a.ctx, events.MiniMode, false)

	// Start watching maximized window position/size
	a.startMaximizedPositionWatch()

	logger.Infof("[App] Restored normal mode")
}

// SetMiniModeExpanded resizes the mini-mode window between compact and full control strip.
func (a *App) SetMiniModeExpanded(expanded bool, height int) {
	if !a.isMiniMode {
		return
	}

	targetW := miniModeCollapsedW
	if expanded {
		targetW = miniModeExpandedW
	}

	targetH := height
	if targetH <= 0 {
		targetH = miniModeCollapsedH
	}

	// Tween window size smoothly.
	startW, startH := runtime.WindowGetSize(a.ctx)
	if startH <= 0 {
		startH = targetH
	}
	if startW == targetW && startH == targetH {
		return
	}

	a.miniResizeMu.Lock()
	if a.miniResizeCancel != nil {
		a.miniResizeCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.miniResizeCancel = cancel
	a.miniResizeMu.Unlock()

	startX, startY := runtime.WindowGetPosition(a.ctx)

	go func(startW int, targetW int, startH int, targetH int, startX int, startY int, ctx context.Context) {
		const steps = 10
		const stepDelay = 12 * time.Millisecond

		for i := 1; i <= steps; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			t := float64(i) / float64(steps)
			w := int(float64(startW) + (float64(targetW-startW) * t))
			h := int(float64(startH) + (float64(targetH-startH) * t))

			// Shift Y coordinate to keep the bottom edge of the window anchored
			diffH := h - startH
			y := startY - diffH

			runtime.WindowSetSize(a.ctx, w, h)
			runtime.WindowSetPosition(a.ctx, startX, y)
			time.Sleep(stepDelay)
		}

		// Snap to the exact target to avoid rounding drift.
		select {
		case <-ctx.Done():
			return
		default:
			diffH := targetH - startH
			runtime.WindowSetSize(a.ctx, targetW, targetH)
			runtime.WindowSetPosition(a.ctx, startX, startY-diffH)
		}
	}(startW, targetW, startH, targetH, startX, startY, ctx)
}

// startPositionWatch starts a goroutine to poll and save window position
func (a *App) startPositionWatch() {
	if a.positionWatchCancel != nil {
		a.positionWatchCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.positionWatchCancel = cancel

	lastSavedX, lastSavedY := a.config.GetMiniModePosition()
	dirty := false
	var lastPersist time.Time

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				if dirty {
					a.config.Save()
				}
				return
			case <-ticker.C:
				rx, ry := runtime.WindowGetPosition(a.ctx)
				_, rh := runtime.WindowGetSize(a.ctx)

				// Calculate baseline Y as if the height was standard collapsed height
				baselineY := ry
				if rh > 0 {
					baselineY = ry + (rh - miniModeCollapsedH)
				}

				if rx != lastSavedX || baselineY != lastSavedY {
					lastSavedX, lastSavedY = rx, baselineY
					a.config.SetMiniModePosition(rx, baselineY)
					dirty = true
				}

				if dirty && time.Since(lastPersist) > 5*time.Second {
					a.config.Save()
					lastPersist = time.Now()
					dirty = false
				}
			}
		}
	}()
}

// saveCurrentMiniModePosition saves the current window position to config if in mini mode
func (a *App) saveCurrentMiniModePosition() {
	if a.isMiniMode {
		x, y := runtime.WindowGetPosition(a.ctx)
		_, h := runtime.WindowGetSize(a.ctx)
		baselineY := y
		if h > 0 {
			baselineY = y + (h - miniModeCollapsedH)
		}
		a.config.SetMiniModePosition(x, baselineY)
		a.config.Save()
		logger.Infof("[App] Saved baseline mini mode position: %d, %d (actual: %d, %d, height: %d)", x, baselineY, x, y, h)
	}
}

// startMaximizedPositionWatch starts a goroutine to poll and save maximized window position and size
func (a *App) startMaximizedPositionWatch() {
	if a.positionWatchCancel != nil {
		a.positionWatchCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.positionWatchCancel = cancel

	lastSavedX, lastSavedY := a.config.GetMaximizedWindowPosition()
	lastSavedW, lastSavedH := a.config.GetMaximizedWindowSize()
	dirty := false
	var lastPersist time.Time

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				if dirty {
					a.config.Save()
				}
				return
			case <-ticker.C:
				rx, ry := runtime.WindowGetPosition(a.ctx)
				rw, rh := runtime.WindowGetSize(a.ctx)

				if rx != lastSavedX || ry != lastSavedY || rw != lastSavedW || rh != lastSavedH {
					lastSavedX, lastSavedY = rx, ry
					lastSavedW, lastSavedH = rw, rh
					a.config.SetMaximizedWindowPosition(rx, ry)
					a.config.SetMaximizedWindowSize(rw, rh)
					dirty = true
				}

				if dirty && time.Since(lastPersist) > 5*time.Second {
					a.config.Save()
					lastPersist = time.Now()
					dirty = false
				}
			}
		}
	}()
}
