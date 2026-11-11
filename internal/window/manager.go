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

// The workspace and native overlay have independent lifetimes and geometry.
func (m *Manager) saveFrameLocked() {
	if m.isMiniMode {
		return
	}
	x, y := runtime.WindowGetPosition(m.ctx)
	w, h := runtime.WindowGetSize(m.ctx)
	m.config.SetMaximizedWindowPosition(x, y)
	m.config.SetMaximizedWindowSize(w, h)
}

func (m *Manager) StartupMiniMode() {
	setChrome(false)
	runtime.WindowHide(m.ctx)
}

func (m *Manager) ShowMini() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isMiniMode {
		return
	}
	m.saveFrameLocked()
	m.config.Save()
	m.isMiniMode = true
	m.userExplicitlyMaximized = false
	runtime.WindowHide(m.ctx)
	setChrome(false)
	runtime.EventsEmit(m.ctx, events.MiniMode, true)
	logger.Infof("[Window] Workspace hidden; running in menu bar")
}

func (m *Manager) HideMini() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isMiniMode {
		m.isMiniMode = false
		m.userExplicitlyMaximized = true
		savedX, savedY := m.config.GetMaximizedWindowPosition()
		savedW, savedH := m.config.GetMaximizedWindowSize()
		runtime.WindowSetMinSize(m.ctx, 640, 480)
		runtime.WindowSetMaxSize(m.ctx, 0, 0)
		if savedW >= 640 && savedH >= 480 {
			runtime.WindowSetSize(m.ctx, savedW, savedH)
		} else {
			runtime.WindowSetSize(m.ctx, 900, 600)
		}
		if savedX != 0 || savedY != 0 {
			runtime.WindowSetPosition(m.ctx, savedX, savedY)
		} else {
			runtime.WindowCenter(m.ctx)
		}
		runtime.EventsEmit(m.ctx, events.MiniMode, false)
	}
	ResetBehavior()
	setChrome(true)
	runtime.WindowShow(m.ctx)
	ConstrainWindow()
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

	runtime.WindowSetMinSize(m.ctx, 640, 480)
	runtime.WindowSetMaxSize(m.ctx, 0, 0)
	runtime.WindowSetSize(m.ctx, 900, 600)
	runtime.WindowCenter(m.ctx)

	m.isMiniMode = false
	m.userExplicitlyMaximized = true
	ResetBehavior()
	setChrome(true)
	runtime.WindowSetAlwaysOnTop(m.ctx, false)
	runtime.EventsEmit(m.ctx, events.MiniMode, false)
	runtime.WindowShow(m.ctx)
	logger.Infof("[Window] Reset window position to center")
}
