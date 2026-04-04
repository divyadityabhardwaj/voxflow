# VoxFlow - Go Backend Study Guide

This guide helps TypeScript developers understand the Go concepts used in VoxFlow, with TypeScript comparisons.

---

## Table of Contents
1. [Project Overview & Architecture](#1-project-overview--architecture)
2. [Go Basics for TypeScript Devs](#2-go-basics-for-typescript-devs)
3. [Frontend-Backend Communication (Wails)](#3-frontend-backend-communication-wails)
4. [Concurrency & Threading](#4-concurrency--threading)
5. [Mutex & Synchronization](#5-mutex--synchronization)
6. [Permissions & System Access](#6-permissions--system-access)
7. [Database (SQLite)](#7-database-sqlite)
8. [Event-Driven Architecture](#8-event-driven-architecture)
9. [HTTP Clients & API Calls](#9-http-clients--api-calls)
10. [Configuration Management](#10-configuration-management)
11. [Dependency Injection](#11-dependency-injection)
12. [Common Interview Questions](#12-common-interview-questions)

---

## 1. Project Overview & Architecture

### What is VoxFlow?
A macOS desktop voice-to-text app built with:
- **Backend**: Go + Wails v2
- **Frontend**: React + TypeScript + TailwindCSS
- **STT Engine**: whisper.cpp (local transcription)
- **LLM Providers**: Gemini, Groq, OpenRouter, Cerebras, Local (Ollama/GGUF)
- **Database**: SQLite

### Architecture Overview
```
┌─────────────────────────────────────────────────────────────┐
│                     React Frontend (TS)                     │
│   MainView | HistoryView | SettingsView | RecordingIndicator│
└─────────────────────┬───────────────────────────────────────┘
                     │ Wails Bridge (IPC)
┌─────────────────────▼───────────────────────────────────────┐
│                     Go Backend                               │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐    │
│  │  Audio   │ │ Hotkey   │ │ Whisper  │ │ LLM Clients  │    │
│  │ Recorder │ │ Manager  │ │ Service  │ │ (Gemini,etc) │    │
│  └──────────┘ └──────────┘ └──────────┘ └──────────────┘    │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────┐    │
│  │ Injection│ │ History  │ │  Config  │ │   Events     │    │
│  │ Service  │ │ Service  │ │ Service  │ │   System     │    │
│  └──────────┘ └──────────┘ └──────────┘ └──────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. Go Basics for TypeScript Devs

### TypeScript → Go Comparison

| TypeScript | Go | Notes |
|------------|-----|-------|
| `interface` | `struct` | Go uses structs, no interfaces needed for simple data |
| `class` | `struct` + methods | Go doesn't have classes, methods on structs instead |
| `const x: Type` | `var x Type` | Go uses `var` for all variables |
| `function()` | `func()` | Functions are declared with `func` keyword |
| `export` | No keyword | Functions/vars starting with uppercase are exported |
| `private` | lowercase | Uncapitalized names are package-private |
| `null` | `nil` | Go uses `nil` for null/uninitialized pointers |
| `async/await` | Goroutines + channels | Go has built-in concurrency |
| `Promise` | `chan` (channel) | Channels are Go's async communication primitive |
| `undefined` | zero values | Go initializes everything to zero value (0, "", nil) |
| `?.` (optional chaining) | comma-ok idiom | Go checks if pointer is nil before accessing |

### Code Example: Struct vs Interface

**TypeScript:**
```typescript
interface Transcript {
  id: number;
  rawText: string;
  polishedText: string;
}

const transcript: Transcript = {
  id: 1,
  rawText: "hello world",
  polishedText: "Hello world."
};
```

**Go (this project - `internal/history/service.go:14-25`):**
```go
type Transcript struct {
  ID                int64     `json:"id"`
  Timestamp         time.Time `json:"timestamp"`
  AppName           string    `json:"app_name"`
  RawText           string    `json:"raw_text"`
  PolishedText      string    `json:"polished_text"`
  Mode              string    `json:"mode"`
  LLMProvider       string    `json:"llm_provider"`
  LLMModel          string    `json:"llm_model"`
  TranslationTimeMs int64     `json:"translation_time_ms"`
  TokensPerSecond   float64   `json:"tokens_per_second"`
}
```

### Tags (backticks)
Go uses struct tags for JSON serialization, similar to TypeScript decorators:
```go
type Transcript struct {
  ID int64 `json:"id"`  // Serializes as "id" in JSON
}
```

---

## 3. Frontend-Backend Communication (Wails)

### How It Works
Wails creates a bridge between Go and JavaScript. The Go backend exposes methods that the frontend calls directly.

### Backend: Exposing Methods (`app.go`)

```go
// app.go:83-87 - Exposed method (capitalized = exported)
func (a *App) SetGeminiModel(model string) error {
  a.config.SetGeminiModel(model)
  a.geminiClient.SetModel(model)
  return a.config.Save()
}
```

### Frontend: Calling Go Methods

**TypeScript (this project - `frontend/src/App.tsx:12-13`):**
```typescript
import { IsMiniMode, ShowMiniMode } from "../wailsjs/go/main/App";

// Calling Go method from React
IsMiniMode().then((isMini) => {
  setIsMiniMode(isMini);
});
```

### Generated Bindings
Wails auto-generates:
- `wailsjs/go/main/App.js` - JavaScript bindings
- `wailsjs/go/main/App.d.ts` - TypeScript type definitions

### Key Wails Concepts

| Concept | Description |
|---------|-------------|
| `runtime.EventsEmit` | Send events from Go to JS (like `emit` in Socket.io) |
| `runtime.EventsOn` | Listen for events in JS (like `on` in Socket.io) |
| `Bind` in options | Tells Wails which Go structs to expose to frontend |

### Example: Bidirectional Communication

**Go emits event (`app.go:358-361`):**
```go
runtime.EventsEmit(a.ctx, events.ModelStatus, map[string]interface{}{
  "downloaded": false,
  "model":      modelSize,
})
```

**React listens for event (`App.tsx:159-170`):**
```typescript
EventsOn(
  Events.ModelStatus,
  (status: { downloaded: boolean; loaded: boolean }) => {
    if (status.downloaded && status.loaded) {
      setModelReady(true);
    }
  },
);
```

---

## 4. Concurrency & Threading

### Go's Concurrency Model

Go is **concurrent by design** - it can run many goroutines (lightweight threads) on few OS threads.

**Key differences from TypeScript:**
- TypeScript: Single-threaded (unless using Web Workers)
- Go: Multiple goroutines can run in parallel

### Goroutines (`go` keyword)

**TypeScript async:**
```typescript
async function processAudio() {
  const result = await transcribe();
  return result;
}
```

**Go goroutine (this project - `app.go:691`):**
```go
// Starts async processing - doesn't block
go a.processRecording()
```

**Go goroutine with callback (`app.go:670-675`):**
```go
go func() {
  vol := audio.MuteSystemAudio()
  a.volumeMu.Lock()
  a.savedVolume = vol
  a.volumeMu.Unlock()
}()
```

### Goroutine vs TypeScript async

| TypeScript | Go |
|------------|-----|
| `async function()` | `func() { go someFunction() }` |
| `await` | `result := <-channel` or blocking call |
| `Promise.all()` | `go func() { ... }` for each + wait groups |

### Concurrency in VoxFlow

**Main recording flow (`app.go:452-472`):**
```go
func (a *App) onHotkeyPressed(state hotkey.State) {
  a.state = state
  runtime.EventsEmit(a.ctx, events.StateChanged, state.String())

  switch state {
  case hotkey.StateRecording:
    if !a.userExplicitlyMaximized {
      a.ShowMiniMode()
    }
    a.StartRecording()  // Blocking
  case hotkey.StateProcessing:
    a.StopRecording()   // Blocking
    go a.processRecording()  // Non-blocking
  case hotkey.StateIdle:
    if !a.userExplicitlyMaximized {
      a.HideMiniMode()
    }
  }
}
```

---

## 5. Mutex & Synchronization

### Why Mutex?
When multiple goroutines access shared data, you need synchronization to prevent race conditions.

### TypeScript: No native mutex (but you can use locks)
```typescript
// TypeScript is single-threaded, no mutex needed for most cases
// But for Web Workers, you'd use SharedArrayBuffer or message passing
```

### Go Mutex (`sync.Mutex`)

**This project - `app.go:51-56`:**
```go
type App struct {
  // ...
  downloadMu        sync.Mutex         // Protects download operations
  downloadCancel    context.CancelFunc // Cancel function for active download
  // ...
  volumeMu          sync.Mutex         // Guards savedVolume
  savedVolume       int                // System volume saved before muting
}
```

### Mutex Usage (`app.go:1050-1060`):
```go
func (a *App) DownloadModelByName(modelName string) error {
  a.downloadMu.Lock()  // Acquire lock

  // Cancel any existing download
  if a.downloadCancel != nil {
    a.downloadCancel()
  }

  // Create new context with cancel
  ctx, cancel := context.WithCancel(context.Background())
  a.downloadCancel = cancel
  a.downloadMu.Unlock()  // Release lock

  // ... rest of download logic
}
```

### Types of Mutex

| Type | Use Case | This Project Example |
|------|----------|---------------------|
| `sync.Mutex` | Exclusive access | `downloadMu`, `volumeMu` |
| `sync.RWMutex` | Many readers, few writers | Config reads vs writes |
| `atomic` | Simple counter/flag | `recording.Load()`, `initialized.Store()` |

### Atomic Operations (`internal/audio/recorder.go:27`):
```go
type Recorder struct {
  // ...
  recording   atomic.Bool  // Thread-safe boolean
  // ...
}

// Check without lock
if r.recording.Load() {
  return "already recording"
}
```

### sync.Once - Run Once Only

**This project - `internal/audio/recorder.go:45-52`:**
```go
func (r *Recorder) Initialize() error {
  r.initOnce.Do(func() {
    r.initErr = portaudio.Initialize()
    if r.initErr == nil {
      r.initialized.Store(true)
    }
  })
  return r.initErr
}
```

Use `sync.Once` for one-time initialization (like lazy singletons in TypeScript).

### Mutex Best Practices

1. **Always unlock** - Use `defer` to ensure release:
```go
func (f *Foo) Bar() {
  f.mu.Lock()
  defer f.mu.Unlock()
  // ... work
}
```

2. **Never hold lock while doing I/O** - Release before network/disk calls
3. **Use RWMutex for read-heavy** - Allows multiple readers

---

## 6. Permissions & System Access

### macOS Accessibility Permission

VoxFlow needs **Accessibility permission** to simulate keyboard input (Cmd+V paste).

**This project - `internal/injection/inject_darwin.go:36-45`:**
```go
// #cgo CFLAGS: -x objective-c
// #include <CoreGraphics/CoreGraphics.h>
// #include <ApplicationServices/ApplicationServices.h>

// checkAccessibility returns 1 if Accessibility access is granted
static int checkAccessibility() {
    return AXIsProcessTrusted() ? 1 : 0;
}

// promptAccessibility shows the system dialog
static void promptAccessibility() {
    NSDictionary *options = @{ (__bridge NSString*)kAXTrustedCheckOptionPrompt: @YES };
    AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
}
```

### Checking Permission (`app.go:288-295`):
```go
// Check and request Accessibility permission needed for CGEventPost (Cmd+V simulation).
// This is a one-time prompt — once granted it persists for the app bundle.
if !injection.IsAccessibilityGranted() {
  fmt.Println("[Injection] Accessibility permission not granted — prompting user")
  injection.PromptAccessibility()
} else {
  fmt.Println("[Injection] Accessibility permission granted")
}
```

### Permission Flow
1. App checks `AXIsProcessTrusted()`
2. If false, calls `AXIsProcessTrustedWithOptions()` to prompt user
3. User grants in System Preferences → Privacy & Security → Accessibility
4. App can now use `CGEventPost` to simulate keystrokes

### TypeScript Comparison
TypeScript in browser can't access system permissions - it relies on browser APIs. Electron apps (which Wails replaces) would use Node.js `child_process`.

---

## 7. Database (SQLite)

### Using SQLite in Go

**This project - `internal/history/service.go:39-49`:**
```go
import (
  "database/sql"
  _ "modernc.org/sqlite"  // Driver import (blank identifier)
)

type Service struct {
  db *sql.DB
}

func NewService() (*Service, error) {
  dbPath, err := getDBPath()
  if err != nil {
    return nil, err
  }

  db, err := sql.Open("sqlite", dbPath)  // Open connection
  if err != nil {
    return nil, fmt.Errorf("failed to open database: %w", err)
  }

  s := &Service{db: db}
  if err := s.initDB(); err != nil {
    return nil, err
  }
  return s, nil
}
```

### Schema Creation (`internal/history/service.go:66-81`):
```go
func (s *Service) initDB() error {
  query := `
  CREATE TABLE IF NOT EXISTS transcripts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    app_name TEXT,
    raw_text TEXT NOT NULL,
    polished_text TEXT,
    mode TEXT,
    llm_provider TEXT,
    llm_model TEXT,
    translation_time_ms INTEGER,
    tokens_per_second REAL
  );
  CREATE INDEX IF NOT EXISTS idx_timestamp ON transcripts(timestamp DESC);
  `
  _, err := s.db.Exec(query)
  return err
}
```

### CRUD Operations

**Insert (`service.go:104-118`):**
```go
func (s *Service) Save(appName, rawText, polishedText, mode, provider, model string, timeMs int64, tps float64) (*Transcript, error) {
  result, err := s.db.Exec(
    "INSERT INTO transcripts (...) VALUES (...)",
    appName, rawText, polishedText, mode, provider, model, timeMs, tps,
  )
  if err != nil {
    return nil, fmt.Errorf("failed to save transcript: %w", err)
  }

  id, err := result.LastInsertId()
  return s.GetByID(id)
}
```

**Query (`service.go:156-161`):**
```go
func (s *Service) GetAll(limit int) ([]*Transcript, error) {
  query := "SELECT ... FROM transcripts ORDER BY timestamp DESC"
  if limit > 0 {
    query += fmt.Sprintf(" LIMIT %d", limit)
  }

  rows, err := s.db.Query(query)
  defer rows.Close()  // Always close!

  var transcripts []*Transcript
  for rows.Next() {
    t := &Transcript{}
    // ... scan into t
    transcripts = append(transcripts, t)
  }
  return transcripts, rows.Err()
}
```

### TypeScript/Node Comparison

| Node.js/SQL | Go |
|-------------|-----|
| `npm install better-sqlite3` | `import _ "modernc.org/sqlite"` |
| `db.prepare()` | `db.Exec()`, `db.Query()` |
| `stmt.run()` | `result, err := db.Exec()` |
| `stmt.all()` | `rows, err := db.Query()` + `rows.Scan()` |

---

## 8. Event-Driven Architecture

### Wails Events

Events allow **Go → JavaScript** communication (reverse of method calls).

**Event Definitions (`internal/events/events.go`):**
```go
package events

const (
  StateChanged          = "state-changed"
  RecordingStarted      = "recording-started"
  RecordingStopped      = "recording-stopped"
  ProcessingComplete    = "processing-complete"
  MiniMode              = "mini-mode"
  ModelStatus           = "model-status"
  ModelDownloadProgress = "model-download-progress"
  Toast                 = "toast"
  Error                 = "error"
  OpenHistory           = "open-history"
  OpenSettings          = "open-settings"
)
```

### Emitting Events (Go → JS)

**From `app.go:358-361`:**
```go
runtime.EventsEmit(a.ctx, events.ModelStatus, map[string]interface{}{
  "downloaded": false,
  "model":      modelSize,
})
```

### Listening (JS → Go direction)

**From React (`App.tsx:142-156`):**
```typescript
import { EventsOn } from "../wailsjs/runtime/runtime";
import { Events } from "./constants/events";

// Listen to events from Go
EventsOn(Events.Toast, (data) => {
  showToast(data.message, data.type);
});

EventsOn(Events.MiniMode, (isMini: boolean) => {
  setIsMiniMode(isMini);
});
```

### Event-Driven Flow Example

```
User presses hotkey
        │
        ▼
┌───────────────────┐
│  Go: onHotkey     │
│  State: Recording │
└────────┬──────────┘
         │ EventsEmit("state-changed", "Recording")
         ▼
┌───────────────────┐
│  React: EventsOn  │
│  Update UI        │
└───────────────────┘
```

### TypeScript Comparison

| Socket.io / EventEmitter | Wails Events |
|-------------------------|--------------|
| `socket.emit('event', data)` | `runtime.EventsEmit(ctx, 'event', data)` |
| `socket.on('event', cb)` | `EventsOn('event', cb)` |
| Bidirectional | Go → JS only (JS → Go via method calls) |

---

## 9. HTTP Clients & API Calls

### Go's net/http

**This project - `internal/gemini/client.go:36-39`:**
```go
type Client struct {
  apiKey     string
  modelName  string
  httpClient *http.Client  // Reusable HTTP client
  models     []string
  modelsMu   sync.Mutex
}

func NewClient(apiKey string, modelName string) *Client {
  return &Client{
    apiKey:    apiKey,
    modelName: modelName,
    httpClient: &http.Client{
      Timeout: 30 * time.Second,
    },
  }
}
```

### Making API Calls

**This project - `gemini/client.go:111-156`:**
```go
func (c *Client) RefineText(rawText string, mode string) (string, int, error) {
  // 1. Build request
  req := Request{ /* ... */ }
  reqBody, err := json.Marshal(req)

  // 2. Create HTTP request
  url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseAPIURL, c.modelName, c.apiKey)
  httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
  httpReq.Header.Set("Content-Type", "application/json")

  // 3. Send request
  resp, err := c.httpClient.Do(httpReq)  // Returns *http.Response
  defer resp.Body.Close()  // Always close!

  // 4. Read and parse response
  respBody, err := io.ReadAll(resp.Body)
  var geminiResp Response
  json.Unmarshal(respBody, &geminiResp)

  // 5. Extract result
  result := geminiResp.Candidates[0].Content.Parts[0].Text
  return result, tokenCount, nil
}
```

### TypeScript Comparison

| TypeScript (fetch) | Go (net/http) |
|-------------------|---------------|
| `fetch(url, {method: 'POST', body: JSON.stringify(req)})` | `http.NewRequest("POST", url, bytes.NewReader(reqBody))` |
| `const resp = await fetch(...)` | `resp, err := client.Do(httpReq)` |
| `const data = await resp.json()` | `json.Unmarshal(respBody, &response)` |

### JSON Handling

**TypeScript:**
```typescript
interface Response {
  candidates: Candidate[];
}

const resp = await fetch(url, options);
const data: Response = await resp.json();
```

**Go:**
```go
type Response struct {
  Candidates    []Candidate    `json:"candidates"`
  UsageMetadata *UsageMetadata `json:"usageMetadata,omitempty"`
}

var resp Response
json.Unmarshal(respBody, &resp)
```

---

## 10. Configuration Management

### Singleton Pattern with sync.Once

**This project - `internal/config/config.go:64-76`:**
```go
var (
  instance *Config
  once     sync.Once  // Ensures init runs only once
)

func GetInstance() *Config {
  once.Do(func() {
    instance = &Config{
      HandsFreeHotkey:  "cmd+shift+space",
      PushToTalkHotkey: "cmd+shift+p",
      WhisperModel:     "base",
      Mode:             "casual",
    }
    instance.Load()  // Load from disk
  })
  return instance
}
```

### TypeScript Equivalent

```typescript
class Config {
  private static instance: Config;
  private constructor() {
    // Load config
  }
  
  static getInstance(): Config {
    if (!Config.instance) {
      Config.instance = new Config();
    }
    return Config.instance;
  }
}
```

### Config File Operations

**Load (`config.go:79-100`):**
```go
func (c *Config) Load() error {
  c.mu.Lock()
  defer c.mu.Unlock()

  configPath, err := GetConfigPath()
  data, err := os.ReadFile(configPath)  // Node: fs.readFileSync
  json.Unmarshal(data, c)
  // ... set defaults if missing
  return nil
}
```

**Save (`config.go:156-171`):**
```go
func (c *Config) Save() error {
  c.mu.RLock()
  defer c.mu.RUnlock()

  data, err := json.MarshalIndent(c, "", "  ")
  return os.WriteFile(configPath, data, 0600)  // Node: fs.writeFileSync
}
```

### Environment Variables

**This project (`config.go:138-150`):**
```go
// Check environment variable first for API key
if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
  c.GeminiAPIKey = apiKey
}
```

---

## 11. Dependency Injection

### Manual DI (Simple Approach)

**This project - `app.go:60-79`:**
```go
func NewApp() *App {
  cfg := config.GetInstance()  // Get singleton config
  
  app := &App{
    config:           cfg,
    state:            hotkey.StateIdle,
    isMiniMode:       true,
    audioRecorder:    audio.NewRecorder(),           // Create instance
    whisperService:   whisper.NewService(),          // Create instance
    localGGUFService: localgguf.NewService(),        // Create instance
    ollamaService:    ollama.NewService(),           // Create instance
    geminiClient:     gemini.NewClient(cfg.GetGeminiAPIKey(), cfg.GetGeminiModel()),
    openRouterClient: openrouter.NewClient(cfg.GetOpenRouterAPIKey()),
    groqClient:       groq.NewClient(cfg.GetGroqAPIKey()),
    cerebrasClient:   cerebras.NewClient(cfg.GetCerebrasAPIKey()),
    savedVolume:      -1,
  }
  
  app.localClient = localclient.NewClient(app.ollamaService.BaseURL())
  return app
}
```

### TypeScript Comparison

```typescript
// Similar manual DI in TypeScript
class App {
  constructor(
    private config: Config,
    private audioRecorder: AudioRecorder,
    private whisperService: WhisperService,
    // ...
  ) {}
}

// Usage
const app = new App(
  Config.getInstance(),
  new AudioRecorder(),
  new WhisperService(),
  // ...
);
```

---

## 12. Common Interview Questions

### Q1: How does frontend communicate with Go backend?

**Answer:** Via Wails bindings. The frontend imports auto-generated JavaScript functions from `wailsjs/go/main/App.js` that call Go methods directly over IPC. Additionally, Go can push events to the frontend using `runtime.EventsEmit()`.

### Q2: How did you handle permissions in this app?

**Answer:** Used macOS Accessibility API. The app checks `AXIsProcessTrusted()` and prompts the user if needed. Once granted, it uses CoreGraphics `CGEventPost` to simulate Cmd+V keystrokes. This is done in `internal/injection/inject_darwin.go` using CGO.

### Q3: Is the app multi-threaded or single-threaded?

**Answer:** Go uses goroutines (lightweight threads). The app is concurrent:
- Main goroutine handles UI/window
- Audio recording runs in a separate goroutine (`go r.readLoop()`)
- Processing runs in background (`go a.processRecording()`)
- Multiple goroutines handle concurrent downloads

Go's goroutines are multiplexed onto OS threads - you get concurrency without managing threads manually.

### Q4: How do you prevent race conditions?

**Answer:** Using mutexes (`sync.Mutex`, `sync.RWMutex`) and atomic operations (`sync/atomic`).
- `downloadMu sync.Mutex` - Protects download state
- `volumeMu sync.Mutex` - Protects savedVolume
- `sync.Once` - For one-time initialization
- `atomic.Bool` - For simple flags

Example from app.go:
```go
a.downloadMu.Lock()
if a.downloadCancel != nil {
  a.downloadCancel()
}
a.downloadCancel = cancel
a.downloadMu.Unlock()
```

### Q5: How does the app persist data?

**Answer:** SQLite via `modernc.org/sqlite` driver. The history service stores transcripts in `~/.voxflow/history.db`.

### Q6: What's the difference between sync.Mutex and sync.RWMutex?

**Answer:** 
- `Mutex`: Only one goroutine can access at a time (both read/write)
- `RWMutex`: Multiple readers OR one writer. Use when you have many reads, few writes

### Q7: How do you handle async operations in Go vs TypeScript?

**Answer:** 
- TypeScript: `async/await`, Promises
- Go: Goroutines (`go func()`) + channels for communication, or blocking calls

### Q8: How does the app handle hotkeys globally?

**Answer:** Using `golang.design/x/hotkey` library. The hotkey manager runs in a main thread loop (via `mainthread.Init`), listening for global keyboard shortcuts even when app isn't focused.

### Q9: Why use sync.Once?

**Answer:** For one-time initialization that must run exactly once, even if called from multiple goroutines. Used in audio recorder initialization - PortAudio should only be initialized once.

### Q10: How do you manage API keys securely?

**Answer:** 
1. Environment variables checked first (env vars take precedence)
2. Stored in config file with `0600` permissions (owner read/write only)
3. API keys never logged or exposed to frontend (only check if set via boolean)

---

## Quick Reference: Go → TypeScript Mapping

```
Go                    TypeScript
────────────────────────────────────
struct                interface / type
func (s *Struct)      class method
sync.Mutex            (no direct equivalent - JS is single-threaded)
sync.Once             let initialized = false; if (!initialized) { ... }
goroutine (go func()) async function
chan                  Promise / EventEmitter
nil                   null / undefined
defer                 try/finally (for cleanup)
interface{}           any
iota                  const enum (sequential values)
```

---

## Key Files to Review

| File | Concept |
|------|---------|
| `app.go` | Main app, mutex usage, goroutines |
| `internal/audio/recorder.go` | Audio capture, mutex, atomic |
| `internal/hotkey/hotkey.go` | Global hotkeys, mainthread |
| `internal/history/service.go` | SQLite CRUD |
| `internal/gemini/client.go` | HTTP client, API calls |
| `internal/config/config.go` | Singleton, file I/O |
| `internal/injection/inject_darwin.go` | CGO, permissions |
| `internal/events/events.go` | Event definitions |
| `frontend/src/App.tsx` | Wails integration |

---

*Generated for VoxFlow Go Implementation Study*
