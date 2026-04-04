# VoxFlow - Interview Q&A

## Go Fundamentals

### Q1: What is the difference between `var` and `:=` in Go?

**Answer:** `var` declares a variable with explicit type, `:=` is short variable declaration that infers type. In this project:

```go
// Explicit type
var configPath string
configPath, err := GetConfigPath()

// Short declaration (inferred)
app := &App{...}  // infers *App
```

---

### Q2: What is the purpose of the `defer` keyword?

**Answer:** `defer` schedules a function to run when the surrounding function returns. Used for cleanup. In this project:

```go
// internal/history/service.go:166
defer rows.Close()  // Always close database rows

// internal/audio/recorder.go:211
defer file.Close()  // Close file after writing
```

**Key point:** `defer` runs even if there's a panic, ensuring resources are cleaned up.

---

### Q3: Explain Go's error handling approach.

**Answer:** Go doesn't have try-catch. Errors are returned as the last return value. In this project:

```go
// app.go:702
wavPath, err := a.audioRecorder.Stop()
if err != nil {
    a.emitToast("Failed to stop recording: "+err.Error(), "error")
    a.resetToIdle()
    return
}
```

**Pattern:** Check error immediately after function call. Use `fmt.Errorf` with `%w` for wrapped errors.

---

### Q4: What is a pointer in Go? When would you use `*` vs `&`?

**Answer:**
- `&` gets the memory address (reference)
- `*` dereferences a pointer or declares a pointer type

```go
// app.go:63 - Creating pointer to struct
app := &App{...}  // Returns *App (pointer to App)

// app.go:84 - Method on pointer receiver
func (a *App) SetGeminiModel(model string) error {
    a.config.SetGeminiModel(model)  // Accessing fields without dereferencing
}
```

**Use pointers when:**
- Modifying the receiver (like `SetGeminiModel`)
- Avoiding copying large structs
- Representing optional values (nil)

---

### Q5: What is the zero value in Go?

**Answer:** Variables are automatically initialized to zero values:
- `0` for numeric types
- `""` for strings
- `nil` for pointers, slices, maps, channels
- `false` for booleans

In this project, config uses defaults:
```go
// config.go:107-118
if c.HandsFreeHotkey == "" {  // "" is zero value for string
    c.HandsFreeHotkey = "cmd+shift+space"
}
```

---

### Q6: What is the difference between `make` and `new`?

**Answer:**
- `new(T)` - Allocates zeroed memory, returns `*T`
- `make(T, args)` - Initializes slices/maps/channels, returns `T`

```go
// new - for pointers
recorder := new(Recorder)

// make - for slices/maps
buffer := make([]int16, 0)  // audio/recorder.go:40
```

---

### Q7: Explain Go's package system.

**Answer:** 
- Files belong to packages
- `package main` = executable, others = libraries
- Exported names start with uppercase
- `import` brings in other packages

```go
// This file is in package main
package main

// Imports
import (
    "context"           // Standard library
    "voxflow/internal/audio"  // Internal package
    "github.com/wailsapp/wails/v2"  // External package
)
```

---

### Q8: What are Go interfaces? How are they used?

**Answer:** Interfaces define method signatures. Types implement implicitly. This project uses interfaces from libraries:

```go
// http.Client interface (standard library)
// Any type with Do(*Request) (*Response, error) satisfies it
httpResp, err := c.httpClient.Do(httpReq)  // c.httpClient is *http.Client
```

**Common interfaces:**
- `io.Reader` - Read bytes
- `io.Closer` - Close resource
- `error` - Has Error() string method

---

## Concurrency

### Q9: What is a goroutine? How is it different from a thread?

**Answer:** A goroutine is a lightweight thread managed by Go runtime, not OS. Thousands can run on few OS threads.

```go
// Starting a goroutine
go a.processRecording()  // app.go:691
go func() { ... }()     // app.go:670 - mute audio in background
```

**Differences from threads:**
| Thread | Goroutine |
|--------|-----------|
| OS-managed | Go runtime-managed |
| ~1MB stack | Starts at ~2KB, grows dynamically |
| Blocking | Cooperative (with some exceptions) |

---

### Q10: What is a mutex? When would you use it?

**Answer:** A mutex provides exclusive access to shared data. Use when multiple goroutines read/write same variable.

**From this project - app.go:51:**
```go
type App struct {
    downloadMu    sync.Mutex
    downloadCancel context.CancelFunc
    volumeMu      sync.Mutex
    savedVolume   int
}
```

**Usage - app.go:1050:**
```go
func (a *App) DownloadModelByName(modelName string) error {
    a.downloadMu.Lock()           // Acquire lock
    if a.downloadCancel != nil {
        a.downloadCancel()
    }
    ctx, cancel := context.WithCancel(context.Background())
    a.downloadCancel = cancel
    a.downloadMu.Unlock()         // Release lock
    // ... continue with download
}
```

---

### Q11: What is the difference between `sync.Mutex` and `sync.RWMutex`?

**Answer:**
- `Mutex` - Only one goroutine at a time (read OR write)
- `RWMutex` - Multiple readers OR one writer

**When to use RWMutex:** When you have many reads, few writes.

```go
// Config uses RWMutex - many reads (every recording), occasional writes (settings change)
type Config struct {
    mu sync.RWMutex  // Many concurrent readers allowed
    // fields...
}

func (c *Config) GetGeminiAPIKey() string {
    c.mu.RLock()        // Read lock - multiple goroutines can read
    defer c.mu.RUnlock()
    return c.GeminiAPIKey
}
```

---

### Q12: What is `sync.Once`? When would you use it?

**Answer:** Ensures a function runs exactly once, even if called from multiple goroutines.

**From this project - audio/recorder.go:45:**
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

**Use case:** One-time initialization (PortAudio should only init once).

---

### Q13: What are atomic operations in Go?

**Answer:** For simple operations on counters/flags without mutex overhead.

**From this project - audio/recorder.go:27:**
```go
type Recorder struct {
    recording   atomic.Bool  // Thread-safe boolean
    initialized atomic.Bool
}

// Usage - no lock needed
if r.recording.Load() {
    return fmt.Errorf("already recording")
}
r.recording.Store(true)
```

---

### Q14: What is a channel in Go?

**Answer:** Channels are pipes for communication between goroutines. Like a typed message queue.

```go
// Built-in, not heavily used in this project, but fundamental concept
ch := make(chan string)
go func() { ch <- "hello" }()
msg := <-ch  // Block until message received
```

---

## Project Architecture

### Q15: Explain the architecture of this project.

**Answer:** 
```
┌─────────────────────────────────────────┐
│     React + TypeScript Frontend         │
│  (Vite, TailwindCSS, Wails bindings)    │
└─────────────────┬───────────────────────┘
                  │ Wails IPC
┌─────────────────▼───────────────────────┐
│         Go Backend (Wails)              │
│  ┌────────────────────────────────────┐ │
│  │ Audio    │ Hotkey │ Whisper │ LLM  │ │
│  │ Recorder │Manager │Service  │Client │ │
│  └────────────────────────────────────┘ │
│  ┌────────────────────────────────────┐ │
│  │ History │Config │Injection │Events  │ │
│  │ Service │Service│ Service │ System │ │
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
```

**Tech Stack:**
- Backend: Go + Wails v2
- Frontend: React + TypeScript + TailwindCSS
- Database: SQLite (modernc.org/sqlite)
- STT: whisper.cpp
- LLM: Gemini, Groq, OpenRouter, Cerebras, Ollama

---

### Q16: How does the frontend communicate with Go backend?

**Answer:** Two ways:

1. **Direct method calls** (JS → Go):
```typescript
// frontend/src/App.tsx:12-13
import { IsMiniMode, ShowMiniMode } from "../wailsjs/go/main/App";

IsMiniMode().then((isMini) => setIsMiniMode(isMini));
```

2. **Events** (Go → JS):
```go
// app.go:358
runtime.EventsEmit(a.ctx, events.ModelStatus, map[string]interface{}{
    "downloaded": false,
    "model":      modelSize,
})
```

```typescript
// frontend/App.tsx:159
EventsOn(Events.ModelStatus, (status) => {
    setModelReady(status.downloaded && status.loaded);
});
```

---

### Q17: What is Wails? How does it work?

**Answer:** Wails is a framework for building desktop apps with Go backend and web frontend.

**Key concepts:**
- `runtime` package - Bridge between Go and JS
- `Bind` in app options - Exposes Go structs to JS
- Auto-generated bindings in `wailsjs/go/`
- Uses WebView2 (Windows) / WebKit (macOS)

---

### Q18: Walk me through what happens when user presses the hotkey.

**Answer:**

1. **Hotkey pressed** → `hotkey.Manager` detects via `golang.design/x/hotkey`
2. **State change** → Calls callback `a.onHotkeyPressed(state)`
3. **Recording starts** → `a.StartRecording()` begins audio capture
4. **Events emitted** → `runtime.EventsEmit("state-changed", "Recording")`
5. **UI updates** → React receives event, shows recording indicator
6. **User releases** → `a.StopRecording()` triggers processing
7. **Async processing** → `go a.processRecording()` runs in background
8. **Transcription** → whisper.cpp converts audio to text
9. **LLM refinement** → Sends to Gemini/OpenRouter/etc
10. **Injection** → Simulates Cmd+V to paste text

---

### Q19: How do you handle configuration in this app?

**Answer:** Singleton pattern with `sync.Once`:

```go
// config.go:64
var instance *Config
var once sync.Once

func GetInstance() *Config {
    once.Do(func() {
        instance = &Config{...}
        instance.Load()  // Load from ~/.voxflow/config.json
    })
    return instance
}
```

**Features:**
- Reads from `~/.voxflow/config.json`
- Checks environment variables first (API keys)
- Uses RWMutex for thread-safe reads/writes
- Auto-saves on changes

---

## System Integration

### Q20: How does the app handle macOS permissions?

**Answer:** Uses Accessibility permission for text injection.

**Check permission - inject_darwin.go:36:**
```go
// C function calls macOS API
static int checkAccessibility() {
    return AXIsProcessTrusted() ? 1 : 0;
}
```

**Prompt user - app.go:290:**
```go
if !injection.IsAccessibilityGranted() {
    injection.PromptAccessibility()  // Shows system dialog
}
```

**Simulate paste - inject_darwin.go:14:**
```go
// Uses CGEventPost to simulate Cmd+V
CGEventRef keyDown = CGEventCreateKeyboardEvent(src, (CGKeyCode)9, true);
CGEventSetFlags(keyDown, kCGEventFlagMaskCommand);
CGEventPost(kCGAnnotatedSessionEventTap, keyDown);
```

---

### Q21: What is CGO? Why is it used here?

**Answer:** CGO lets Go call C code. Used for macOS system APIs that don't have Go wrappers.

```go
// inject_darwin.go:1-47
/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework ApplicationServices

#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>

static int simulateCmdV() { ... }
*/
import "C"
```

**Why:** CoreGraphics is C API, not Go. CGO bridges this gap.

---

### Q22: How does the app persist data?

**Answer:** SQLite database at `~/.voxflow/history.db`

```go
// history/service.go:39
db, err := sql.Open("sqlite", dbPath)

// Schema
CREATE TABLE IF NOT EXISTS transcripts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    raw_text TEXT NOT NULL,
    polished_text TEXT,
    mode TEXT,
    llm_provider TEXT,
    ...
);
```

---

### Q23: What is the purpose of goroutines in the audio recording?

**Answer:** Keeps recording non-blocking while UI remains responsive.

```go
// audio/recorder.go:110
go r.readLoop(inputBuffer)  // Runs in background, continuously reads audio

// app.go:670 - Mute system audio without blocking
go func() {
    vol := audio.MuteSystemAudio()
    a.volumeMu.Lock()
    a.savedVolume = vol
    a.volumeMu.Unlock()
}()
```

---

### Q24: How do you prevent race conditions in this app?

**Answer:** Multiple strategies:

1. **Mutex for shared state:**
```go
// app.go:1050
a.downloadMu.Lock()
a.downloadCancel = cancel
a.downloadMu.Unlock()
```

2. **Atomic for simple flags:**
```go
// audio/recorder.go:27
recording atomic.Bool
if r.recording.Load() { ... }
```

3. **Channels for communication:**
```go
// hotkey/hotkey.go:186
select {
case req := <-m.reconfigCh:
    m.handleReconfigure(req.handsFreeStr, req.pttStr)
case <-hfDown:
    m.handleHandsFree()
}
```

4. **sync.Once for initialization:**
```go
// audio/recorder.go:46
r.initOnce.Do(func() { r.initErr = portaudio.Initialize() })
```

---

## HTTP & APIs

### Q25: How does the app make API calls to Gemini?

**Answer:** Using Go's `net/http` package:

```go
// gemini/client.go:146
url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseAPIURL, c.modelName, c.apiKey)
httpReq, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
httpReq.Header.Set("Content-Type", "application/json")

resp, err := c.httpClient.Do(httpReq)  // Send request
defer resp.Body.Close()

respBody, err := io.ReadAll(resp.Body)
json.Unmarshal(respBody, &geminiResp)
```

---

### Q26: Why use a custom HTTP client instead of default?

**Answer:** Configure timeout and reuse connection:

```go
// gemini/client.go:36
httpClient: &http.Client{
    Timeout: 30 * time.Second,  // Prevent hanging
},
```

---

## Database

### Q27: How do you handle database connections safely?

**Answer:**
1. Open on startup, close on shutdown
2. Use `defer rows.Close()` after queries
3. Check errors at every step

```go
// service.go:156
rows, err := s.db.Query(query)
defer rows.Close()  // Always close!

for rows.Next() {
    err := rows.Scan(...)
}
return rows.Err()  // Check iteration errors
```

---

## Design Patterns

### Q28: What design patterns are used in this project?

**Answer:**

1. **Singleton** - Config (sync.Once)
2. **Dependency Injection** - Services injected in NewApp()
3. **Repository** - History service abstracts DB
4. **Factory** - Client constructors (NewClient, NewService)
5. **Observer** - Event system (Wails EventsOn/EventsEmit)

---

## Troubleshooting & Edge Cases

### Q29: How do you handle errors during recording?

**Answer:** Check every step, emit events to UI:

```go
// app.go:653
func (a *App) StartRecording() error {
    if !a.modelReady {
        return fmt.Errorf("model not ready")
    }
    if err := a.audioRecorder.Start(); err != nil {
        a.state = hotkey.StateIdle
        runtime.EventsEmit(a.ctx, events.Error, err.Error())
        return err
    }
    runtime.EventsEmit(a.ctx, events.RecordingStarted, nil)
    return nil
}
```

---

### Q30: What happens if the user closes the app during recording?

**Answer:** Cleanup in shutdown:

```go
// app.go:322
func (a *App) shutdown(ctx context.Context) {
    if a.hotkeyManager != nil {
        a.hotkeyManager.Stop()
    }
    if a.audioRecorder != nil {
        a.audioRecorder.Terminate()  // Stop audio
    }
    if a.whisperService != nil {
        a.whisperService.Close()
    }
    // Save config
    a.config.Save()
}
```

---

## Advanced

### Q31: Explain the difference between goroutines and threads in production.

**Answer:**
- **Threads:** OS-managed, ~1MB stack each, expensive to create
- **Goroutines:** Go-runtime managed, ~2KB initial stack, cheap to create
- Go multiplexes thousands of goroutines onto few OS threads
- Goroutines grow stack dynamically (up to 1GB if needed)

**In VoxFlow:**
- Audio read loop runs in goroutine
- Each model download runs in goroutine
- LLM API calls are blocking but run in goroutine so UI stays responsive

---

### Q32: Why is context.Context used?

**Answer:** For cancellation and timeouts across API calls.

```go
// app.go:1293
func (a *App) ensureLocalModelServer() error {
    ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
    defer cancel()
    return a.ollamaService.EnsureServer(ctx)
}
```

**Use cases:**
- Cancel long-running operations
- Set timeouts
- Propagate cancellation through call chain

---

### Q33: How does the event system work internally?

**Answer:** Wails uses WebSocket-like communication:

1. Go calls `runtime.EventsEmit(ctx, "event-name", data)`
2. Wails serializes data to JSON
3. WebView receives event via internal channel
4. JavaScript callbacks fire

**Event types in this project:**
- State changes (Recording → Processing → Idle)
- Model status (downloaded, loaded, error)
- UI events (toasts, navigation)

---

### Q34: What is the hotkey manager architecture?

**Answer:** Uses mainthread for system event loop:

```go
// hotkey/hotkey.go:162
go mainthread.Init(func() {
    // Register hotkeys
    m.handsFreeHK = hotkey.New(mods, key)
    m.handsFreeHK.Register()

    // Event loop
    for {
        select {
        case req := <-m.reconfigCh:
            m.handleReconfigure(req.handsFreeStr, req.pttStr)
        case <-hfDown:
            m.handleHandsFree()
        case <-pttDown:
            m.handlePushToTalkDown()
        }
    }
})
```

---

## Behavioral

### Q35: What was the most challenging part of this project?

**Possible answer:** Handling macOS permissions and text injection was tricky because:
- Accessibility permission required for CGEventPost
- Can't test without actual macOS environment
- CGO integration needed for CoreGraphics
- Permission doesn't survive dev rebuilds easily

---

### Q36: How did you test this application?

**Answer:** 
- Manual testing during development
- Wails dev mode for hot reload
- Console logging throughout (fmt.Printf)
- Event system for debugging state changes
- No formal unit tests in this codebase (opportunity for improvement)

---

### Q37: How would you improve this if you built it again?

**Answer:** Areas to improve:
1. Add unit tests (especially for business logic)
2. Use dependency injection framework
3. Add logging library (currently using fmt)
4. Consider gRPC for better API contracts
5. Add more error recovery/retry logic

---

### Q38: Why did you choose Go for this backend?

**Answer:**
- **Performance:** Compiled, fast
- **Concurrency:** Native goroutines perfect for audio streaming
- **CGO:** Easy system integration (CoreGraphics, PortAudio)
- **Single binary:** Easy distribution
- **Wails:** Mature framework for Go + web frontend

---

### Q39: How does this compare to Electron?

**Answer:**
| Aspect | Electron | Wails (Go) |
|--------|----------|------------|
| Backend | Node.js | Go |
| Binary size | Large (Node bundled) | Smaller |
| Performance | Good | Excellent |
| System access | Via Node | Direct CGO |
| Bundle | Large | Smaller |
| Memory | Higher | Lower |

---

### Q40: What would you do differently for Windows support?

**Answer:**
- Implement `inject_windows.go` using different APIs (user32.dll)
- Different hotkey implementation (RegisterHotKey)
- Use WebView2 instead of WebKit
- File paths use backslashes
- Currently Windows build files exist in `build/windows/`

---

*End of Interview Questions*
