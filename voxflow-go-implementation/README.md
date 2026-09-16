# VoxFlow (Go Implementation)

**The core desktop application for VoxFlow, built with Go, Wails, and React.**

VoxFlow is a macOS-optimized voice-to-text tool that captures your voice, transcribes it locally, and uses LLMs to refine it into professional, ready-to-use text.

## How It Works

1.  **Global Capture**: A hands-free hotkey (`Cmd+Shift+Space`, press to start and again to stop) or a push-to-talk hotkey (`Cmd+Shift+P`, hold to record) triggers recording. The menu bar icon does the same.
2.  **Audio Processing**: Microphone audio is captured via PortAudio at 16 kHz and streamed in chunks of up to 8 s, each cut at the quietest pause so words are never split.
3.  **Local Transcription**: Chunks are transcribed by `whisper.cpp`, kept resident in a local `whisper-server` process (falling back to `whisper-cli` per call when the server binary is missing). No voice data leaves your machine.
4.  **AI refinement**: The raw text is sent to an LLM (Gemini, OpenRouter, Groq, Cerebras, or a local OpenAI-compatible server such as Ollama) for punctuation, filler removal and formatting. Raw and copy-only modes skip this step entirely.
5.  **Smart Injection**: The polished text is pasted into the frontmost app with a simulated `Cmd+V` (CoreGraphics events), or, per app rule, typed as keystrokes or only copied to the clipboard.

## Project Structure

```text
.
├── main.go               # Entry point, Wails options, native menu
├── app.go, app_*.go      # Methods bound to the frontend (config, history, models, window, rules)
├── internal/
│   ├── orchestrator/     # Recording → transcription → refinement → injection pipeline
│   ├── audio/            # PortAudio capture, pause-aware chunking, system mute
│   ├── whisper/          # Model downloads, whisper-server client, whisper-cli fallback
│   ├── llm/              # Shared prompt, JSON parsing, OpenAI-compatible client, retry
│   ├── gemini/, groq/, cerebras/, openrouter/, localclient/   # Providers
│   ├── injection/        # Cmd+V paste, keystroke typing, clipboard (CoreGraphics)
│   ├── hotkey/           # Global shortcuts and recording state machine
│   ├── window/           # Mini pill / full window management, menu bar status item
│   ├── history/          # SQLite transcript storage and search
│   ├── config/           # ~/.voxflow/config.json and per-app rules
│   └── macos/, logger/, events/
└── frontend/             # React + TypeScript + Tailwind (Vite)
    └── src/components/   # MainView, HistoryView, SettingsView, OnboardingWizard, RecordingIndicator
```

## Tech Stack

- **Framework**: [Wails v2](https://wails.io/) (Go backend, Web frontend)
- **Frontend**: React + TypeScript + Tailwind CSS
- **STT Engine**: [whisper.cpp](https://github.com/ggml-org/whisper.cpp) via a resident `whisper-server` (Metal on Apple Silicon), `whisper-cli` fallback
- **Refinement**: Google Gemini, OpenRouter, Groq, Cerebras, or any local OpenAI-compatible server (Ollama, LM Studio, llama.cpp)
- **Database**: SQLite (via `modernc.org/sqlite`)
- **Injection**: CoreGraphics key events (simulated `Cmd+V` or unicode keystrokes)

## Prerequisites

- **macOS** (Optimized for Apple Silicon)
- **Go 1.24+**
- **Node.js 20.19+**
- **Wails CLI**: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **PortAudio**: `brew install portaudio`
- **whisper.cpp**: `brew install whisper-cpp` (provides `whisper-cli` and `whisper-server`)

## Development

For a simplified setup and launch, use the provided development script:

```bash
chmod +x dev.sh
./dev.sh
```

On first launch, the app will assist in downloading the necessary Whisper models (~142MB for base).

## Configuration

Settings are stored in `~/.voxflow/config.json` (models in `~/.voxflow/models`, transcripts in `~/.voxflow/history.db`, logs in `~/.voxflow/voxflow.log`). From the Settings view you can configure:

- **LLM Provider**: Gemini, OpenRouter, Groq, Cerebras, or a local server, with a connection test per model.
- **Pipeline Mode**: Refine (Whisper → LLM → paste), Raw (paste as transcribed) or Copy only.
- **Hotkeys**: Separate hands-free and push-to-talk shortcuts.
- **Whisper Model and Language**: Balance speed (`tiny`) against accuracy (`medium`); fix the language or auto-detect.
- **Vocabulary**: Terms Whisper and the LLM should spell your way.
- **Per-App Rules**: Override the pipeline mode and the delivery method (paste, typed keystrokes, clipboard) for specific apps.
- **Mute System Audio**: Silence speakers while recording.

## Troubleshooting

### "VoxFlow is damaged and can't be opened"

Because the app is not signed with an Apple Developer Certificate, macOS may block it. To fix this, run:

```bash
xattr -cr /Applications/voxflow.app
```

(Adjust the path if you've moved the app elsewhere)

---

_VoxFlow is a personal project built for speed and privacy. It's designed to make voice the primary input method for developers and power users._
