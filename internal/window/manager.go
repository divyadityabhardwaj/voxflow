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

type Manager struct {
	ctx    context.Context
	config *config.Config

	mu                      sync.Mutex
	isMiniMode              bool
	userExplicitlyMaximized bool
	frameSaveTimer          *time.Timer

	miniResizeMu     sync.Mutex
	miniResizeCancel context.CancelFunc
}

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
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isMiniMode
}

func (m *Manager) UserExplicitlyMaximized() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.userExplicitlyMaximized
}

// WatchFrame saves the window position after the user moves or resizes it.
func (m *Manager) WatchFrame() {
	observeWindowFrame(m)
}

func (m *Manager) frameChanged() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.frameSaveTimer != nil {
		m.frameSaveTimer.Stop()
	}
	m.frameSaveTimer = time.AfterFunc(500*time.Millisecond, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.saveFrameLocked()
		m.config.Save()
	})
}

// saveFrameLocked records the current frame for the current mode. The mini
// position is stored as its collapsed baseline, since the pill grows upward.
func (m *Manager) saveFrameLocked() {
	x, y := runtime.WindowGetPosition(m.ctx)
	w, h := runtime.WindowGetSize(m.ctx)
	if m.isMiniMode {
		if h > 0 {
			y += h - MiniModeCollapsedH
		}
		m.config.SetMiniModePosition(x, y)
		return
	}
	m.config.SetMaximizedWindowPosition(x, y)
	m.config.SetMaximizedWindowSize(w, h)
}

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
}

func (m *Manager) ShowMini() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isMiniMode {
		return
	}
	m.saveFrameLocked()
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

	logger.Infof("[Window] Switched to mini mode")
}

func (m *Manager) HideMini() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.isMiniMode {
		return
	}

	m.saveFrameLocked()
	m.config.Save()

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

	logger.Infof("[Window] Restored normal mode")
}

func (m *Manager) SetMiniExpanded(expanded bool, height int) {
	if !m.IsMiniMode() {
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

func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.frameSaveTimer != nil {
		m.frameSaveTimer.Stop()
	}
	m.saveFrameLocked()
}

func (m *Manager) ResetPosition() {
	m.mu.Lock()
	defer m.mu.Unlock()
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
	logger.Infof("[Window] Reset window position to center")
}
