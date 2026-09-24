# VoxFlow — Product Overview

This document describes what VoxFlow is, how the system is put together, and the design choices that matter if you want more than the README.

## Problem

Typing is often the bottleneck when prompting LLMs, writing docs, or messaging. System dictation exists, but it typically requires speaking punctuation, leaves fillers in place, and does not clean mid-sentence corrections. Cloud-only voice tools raise privacy and latency concerns for everyday use.

VoxFlow targets that gap: speak naturally, get clean text in the app you already have focused — with transcription staying local.

## What it does

VoxFlow is a macOS menu-bar / floating-pill desktop app. You trigger recording with a global hotkey or the menu bar. While you talk, audio is chunked and transcribed on-device with whisper.cpp. Optionally, an LLM polishes the transcript (punctuation, fillers, light cleanup). The result is injected into the previously frontmost application via simulated paste, typed keystrokes, or clipboard-only delivery.

Nothing about speech audio is uploaded for transcription. Only the text (when refinement is enabled) is sent to the configured LLM provider — or to a local server you run yourself.

## User experience

### Recording

- **Hands-free hotkey** (default `Cmd+Shift+Space`): press to start, press again to stop.
- **Push-to-talk** (default `Cmd+Shift+P`): hold to record, release to stop.
- **Menu bar**: start/stop, open main window, settings, quit. Icon reflects idle / recording / processing.

While recording, a compact floating indicator (mini mode) shows status. Expanding the window reveals history, settings, and onboarding.

### Pipeline modes

| Mode | Behavior |
|------|----------|
| **Refine** | Whisper → LLM → inject |
| **Raw** | Whisper → inject (no LLM) |
| **Copy-only** | Produce text and leave it on the clipboard without pasting |

### Per-app rules

Each macOS app can override:

- Refinement mode
- Inject method: **paste** (simulated `Cmd+V`), **type** (unicode keystrokes for apps that remap paste, e.g. vim-mode editors), or **clipboard** (copy only)

The frontmost app is captured when recording starts so rules and paste target stay correct even if focus briefly moves to VoxFlow.

### Vocabulary & language

Custom vocabulary terms are passed as Whisper's initial prompt and included in the LLM system prompt so names and jargon spell consistently. Whisper language can be fixed or set to auto-detect.

### History

Transcripts are stored in SQLite (`~/.voxflow/history.db`): raw text, polished text, app name, provider/model, timing. The UI supports search, pagination, retry with a custom instruction, and re-inject / copy.

## Architecture

```text
┌──────────────────────────────────────────────────────────┐
│  React UI (Vite + TypeScript + Tailwind)                 │
│  Main · History · Settings · Onboarding · Recording pill │
└────────────────────────────┬─────────────────────────────┘
                             │ Wails IPC + events
┌────────────────────────────▼─────────────────────────────┐
│  Go (Wails)                                              │
│                                                          │
│  hotkey ──► orchestrator pipeline                        │
│               │                                          │
│               ├─ audio (PortAudio, pause-aware chunks)   │
│               ├─ whisper (resident server / CLI)         │
│               ├─ llm providers (Gemini, Groq, …)         │
│               ├─ injection (CoreGraphics)                │
│               ├─ history (SQLite)                        │
│               └─ config (~/.voxflow/config.json)         │
│                                                          │
│  window · status item · macos frontmost · events         │
└──────────────────────────────────────────────────────────┘
```

### Pipeline (happy path)

1. Hotkey / menu → `orchestrator` starts recording; optionally mutes system audio.
2. Audio reader accumulates samples; every ~8 s (or on stop) cuts at the quietest recent pause and streams a chunk.
3. Chunks go to `whisper-server` if running, else `whisper-cli`.
4. Partial transcripts are rate-limited to the UI (~10 events/s).
5. On stop, any uncovered tail is transcribed; if still empty but activity was detected, a full-file fallback runs.
6. Effective mode is resolved from global settings + per-app rule for the captured target.
7. If refining, text is sent to the active LLM; on failure, raw text is still injected (losing the dictation is worse than pasting unpolished text).
8. Injection uses paste, type, or clipboard per rule; history save is fire-and-forget off the critical path.

### Why a resident Whisper server

Spawning `whisper-cli` per chunk pays process start, Metal init, and model load (~hundreds of ms). `whisper-server` keeps one model resident so streaming stays interactive. On macOS there is no automatic parent-death signal for the child, so VoxFlow tracks the PID and reaps orphans on next start.

### Text injection

Paste is implemented with CoreGraphics `CGEventPost` (`Cmd+V`), not AppleScript System Events. Accessibility permission is therefore tied to the app process itself (survives typical rebuilds better than osascript-based approaches). Clipboard restore is delayed so Electron apps and terminals that service paste late still read VoxFlow's text.

### Frontend ↔ backend

Wails binds exported `App` methods to TypeScript. Go pushes UI updates with `runtime.EventsEmit` (state changes, model download progress, toasts, mini-mode). Generated bindings live under `frontend/wailsjs/`.

## Data on disk

| Path | Contents |
|------|----------|
| `~/.voxflow/config.json` | Settings, API keys (file mode `0600`), per-app rules |
| `~/.voxflow/models/` | Downloaded Whisper `.bin` models |
| `~/.voxflow/history.db` | Transcript history |
| `~/.voxflow/voxflow.log` | Log file (packaged `.app` has no stdout) |
| `~/.voxflow/` model cache files | Cached provider model lists |

Config saves use write-then-rename so a crash mid-write cannot truncate a file that holds API keys.

## Providers

All cloud OpenAI-compatible providers share a common HTTP client (`internal/llm`) with retry/backoff, connection pre-warm, and shared prompt/JSON parsing. Gemini uses its native API with the key in a header (not query string) to avoid leaking into proxy logs. Local mode talks to any OpenAI-compatible root URL (e.g. `http://localhost:11434`); `/v1` is appended automatically.

Refinement asks the model for structured JSON (`refined_text`, `ok_to_go`, etc.). If the reply is malformed JSON that looks like a truncated object, VoxFlow falls back to the raw transcript rather than pasting a JSON fragment into the user's app.

## Notable design decisions

- **Local STT first** — privacy and predictable latency for the heavy audio path.
- **Streaming with pause-aware cuts** — live feedback without chopping words across chunk boundaries.
- **Fail open on LLM errors** — always deliver something usable.
- **Per-app inject methods** — paste is default; type covers remapped `Cmd+V`; clipboard covers cautious workflows.
- **No second Cocoa main loop for hotkeys** — Wails owns AppKit; nesting another mainthread loop previously drove idle CPU near 100%.
- **Accessibility on the process** — CGEvent paste stays granted across rebuilds better than osascript.

## Current scope & limits

- **Platform**: macOS 11+ on Apple Silicon (CGO / AppKit / CoreGraphics paths). Non-Darwin stubs exist where needed for compile, not as a full Windows/Linux product.
- **Signing**: releases are ad-hoc signed, not Apple-notarized; users approve the first open in Privacy & Security (or run `xattr -cr`), and permissions may need re-granting after an update.
- **Distribution**: GitHub Actions builds a darwin/arm64 DMG on version tags. The release app is self-contained: PortAudio is linked statically, and `whisper-cli` / `whisper-server` (whisper.cpp built with Metal shaders embedded) ship in `voxflow.app/Contents/MacOS/`, where the app looks first. Both are built from source for macOS 11 by `scripts/build-deps.sh`; CI fails the release if any binary links a Homebrew library. Dev builds still use Homebrew's `portaudio` and `whisper-cpp`.

## Related reading

- Root [README](../README.md) — setup, features, layout
- Source of truth for behavior: `internal/orchestrator/`, `internal/whisper/`, `internal/injection/`
