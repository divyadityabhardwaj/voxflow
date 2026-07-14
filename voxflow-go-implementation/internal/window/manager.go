package window

import (
	"context"
	"sync"
	"time"
	"voxflow/internal/config"
	"voxflow/internal/events"
	"voxflow/internal/logger"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Manager handles mini/full window mode transitions and position persistence.
type Manager struct {
	ctx    context.Context
	config *config.Config

	isMiniMode              bool
	userExplicitlyMaximized bool

	positionWatchCancel context.CancelFunc

	miniResizeMu     sync.Mutex
	miniResizeCancel context.CancelFunc
}

// NewManager creates a window manager bound to the Wails runtime context.
func NewManager(ctx context.Context, cfg *config.Config) *Manager {
	return &Manager{
		ctx:        ctx,
		config:     cfg,
		isMiniMode: true,
	}
}

func (m *Manager) Context() context.Context {
	return m.ctx
}

func (m *Manager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

func (m *Manager) IsMiniMode() bool {
	return m.isMiniMode
}

func (m *Manager) UserExplicitlyMaximized() bool {
	return m.userExplicitlyMaximized
}

// StartupMiniMode restores saved mini position and begins position watching.
func (m *Manager) StartupMiniMode() {
	x, y := m.config.GetMiniModePosition()

	runtime.WindowSetMinSize(m.ctx, MiniModeCollapsedW, MiniModeCollapsedH)
	runtime.WindowSetMaxSize(m.ctx, MiniModeExpandedW, MiniModeExpandedH)
	runtime.WindowSetSize(m.ctx, MiniModeCollapsedW, MiniModeCollapsedH)

	if x != 0 || y != 0 {
		runtime.WindowSetPosition(m.ctx, x, y)
	} else {
		runtime.WindowCenter(m.ctx)
	}
	ConstrainWindow()
	m.startPositionWatch()
}

// ShowMini switches the window to the floating indicator.
func (m *Manager) ShowMini() {
	if m.isMiniMode {
		return
	}
	m.isMiniMode = true
	m.userExplicitlyMaximized = false

	FloatEverywhere()

	runtime.WindowSetMinSize(m.ctx, MiniModeCollapsedW, MiniModeCollapsedH)
	runtime.WindowSetMaxSize(m.ctx, MiniModeExpandedW, MiniModeExpandedH)
	runtime.WindowSetSize(m.ctx, MiniModeCollapsedW, MiniModeCollapsedH)

	x, y := m.config.GetMiniModePosition()
	if x != 0 || y != 0 {
		runtime.WindowSetPosition(m.ctx, x, y)
	} else {
		runtime.WindowCenter(m.ctx)
	}
	ConstrainWindow()

	runtime.WindowSetAlwaysOnTop(m.ctx, true)
	runtime.EventsEmit(m.ctx, events.MiniMode, true)
	m.startPositionWatch()

	logger.Infof("[Window] Switched to mini mode")
}

// HideMini restores the window to normal full-app size.
func (m *Manager) HideMini() {
	if !m.isMiniMode {
		return
	}

	if m.positionWatchCancel != nil {
		m.positionWatchCancel()
		m.positionWatchCancel = nil
	}

	m.saveCurrentMiniPosition()

	m.isMiniMode = false
	m.userExplicitlyMaximized = true

	ResetBehavior()

	savedX, savedY := m.config.GetMaximizedWindowPosition()
	savedW, savedH := m.config.GetMaximizedWindowSize()

	runtime.WindowSetMinSize(m.ctx, 800, 600)
	runtime.WindowSetMaxSize(m.ctx, 0, 0)

	if savedW > 0 && savedH > 0 {
		runtime.WindowSetSize(m.ctx, savedW, savedH)
	} else {
		runtime.WindowSetSize(m.ctx, 900, 600)
	}

	if savedX != 0 || savedY != 0 {
		runtime.WindowSetPosition(m.ctx, savedX, savedY)
	} else {
		runtime.WindowCenter(m.ctx)
	}

	runtime.WindowSetAlwaysOnTop(m.ctx, false)
	runtime.EventsEmit(m.ctx, events.MiniMode, false)
	m.startMaximizedPositionWatch()

	logger.Infof("[Window] Restored normal mode")
}

// SetMiniExpanded tweens mini-mode window size between compact and expanded strip.
func (m *Manager) SetMiniExpanded(expanded bool, height int) {
	if !m.isMiniMode {
		return
	}

	targetW := MiniModeCollapsedW
	if expanded {
		targetW = MiniModeExpandedW
	}

	targetH := height
	if targetH <= 0 {
		targetH = MiniModeCollapsedH
	}

	startW, startH := runtime.WindowGetSize(m.ctx)
	if startH <= 0 {
		startH = targetH
	}
	if startW == targetW && startH == targetH {
		return
	}

	m.miniResizeMu.Lock()
	if m.miniResizeCancel != nil {
		m.miniResizeCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.miniResizeCancel = cancel
	m.miniResizeMu.Unlock()

	startX, startY := runtime.WindowGetPosition(m.ctx)

	go func(startW, targetW, startH, targetH, startX, startY int, ctx context.Context) {
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
			diffH := h - startH
			y := startY - diffH

			runtime.WindowSetSize(m.ctx, w, h)
			runtime.WindowSetPosition(m.ctx, startX, y)
			time.Sleep(stepDelay)
		}

		select {
		case <-ctx.Done():
			return
		default:
			diffH := targetH - startH
			runtime.WindowSetSize(m.ctx, targetW, targetH)
			runtime.WindowSetPosition(m.ctx, startX, startY-diffH)
		}
	}(startW, targetW, startH, targetH, startX, startY, ctx)
}

func (m *Manager) startPositionWatch() {
	m.startWindowWatch(true)
}

func (m *Manager) startMaximizedPositionWatch() {
	m.startWindowWatch(false)
}

func (m *Manager) saveCurrentMiniPosition() {
	if m.isMiniMode {
		x, y := runtime.WindowGetPosition(m.ctx)
		_, h := runtime.WindowGetSize(m.ctx)
		baselineY := y
		if h > 0 {
			baselineY = y + (h - MiniModeCollapsedH)
		}
		m.config.SetMiniModePosition(x, baselineY)
		m.config.Save()
		logger.Infof("[Window] Saved baseline mini mode position: %d, %d", x, baselineY)
	}
}

// Shutdown persists mini-mode position when applicable.
func (m *Manager) Shutdown() {
	if m.positionWatchCancel != nil {
		m.positionWatchCancel()
	}
	if m.isMiniMode {
		m.saveCurrentMiniPosition()
	}
}

// ResetPosition centers the window and clears saved positions.
func (m *Manager) ResetPosition() {
	m.config.SetMiniModePosition(0, 0)
	m.config.SetMaximizedWindowPosition(0, 0)
	m.config.SetMaximizedWindowSize(900, 600)
	m.config.Save()

	runtime.WindowSetMinSize(m.ctx, 800, 600)
	runtime.WindowSetMaxSize(m.ctx, 0, 0)
	runtime.WindowSetSize(m.ctx, 900, 600)
	runtime.WindowCenter(m.ctx)

	m.isMiniMode = false
	m.userExplicitlyMaximized = true
	ResetBehavior()
	runtime.WindowSetAlwaysOnTop(m.ctx, false)
	runtime.EventsEmit(m.ctx, events.MiniMode, false)
	m.startMaximizedPositionWatch()
	logger.Infof("[Window] Reset window position to center")
}

func (m *Manager) startWindowWatch(isMini bool) {
	if m.positionWatchCancel != nil {
		m.positionWatchCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.positionWatchCancel = cancel

	var lastSavedX, lastSavedY int
	var lastSavedW, lastSavedH int
	if isMini {
		lastSavedX, lastSavedY = m.config.GetMiniModePosition()
	} else {
		lastSavedX, lastSavedY = m.config.GetMaximizedWindowPosition()
		lastSavedW, lastSavedH = m.config.GetMaximizedWindowSize()
	}

	dirty := false
	var lastPersist time.Time

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				if dirty {
					m.config.Save()
				}
				return
			case <-ticker.C:
				rx, ry := runtime.WindowGetPosition(m.ctx)
				rw, rh := runtime.WindowGetSize(m.ctx)

				if isMini {
					baselineY := ry
					if rh > 0 {
						baselineY = ry + (rh - MiniModeCollapsedH)
					}

					if rx != lastSavedX || baselineY != lastSavedY {
						lastSavedX, lastSavedY = rx, baselineY
						m.config.SetMiniModePosition(rx, baselineY)
						dirty = true
					}
				} else {
					if rx != lastSavedX || ry != lastSavedY || rw != lastSavedW || rh != lastSavedH {
						lastSavedX, lastSavedY = rx, ry
						lastSavedW, lastSavedH = rw, rh
						m.config.SetMaximizedWindowPosition(rx, ry)
						m.config.SetMaximizedWindowSize(rw, rh)
						dirty = true
					}
				}

				if dirty && time.Since(lastPersist) > 5*time.Second {
					m.config.Save()
					lastPersist = time.Now()
					dirty = false
				}
			}
		}
	}()
}
