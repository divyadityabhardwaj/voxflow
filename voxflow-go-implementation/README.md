# Voxflow

**AI-powered voice dictation for macOS** — Speak naturally, get polished text instantly.

Voxflow captures your voice, transcribes it locally using Whisper, refines it with Gemini AI, and pastes the result directly into any application.

## Features

- 🎙️ **Global Hotkey** — Press `Cmd+Shift+V` from any app to start/stop recording
- 🔒 **Privacy-First** — Speech recognition runs locally via Whisper
- ✨ **AI Refinement** — Gemini removes filler words, fixes grammar, follows commands
- 📝 **History Vault** — Search past transcriptions with raw vs polished view
- ⚡ **Fast** — Under 3 seconds from stop to paste

## Prerequisites

- **macOS** (Apple Silicon or Intel)
- **Go 1.21+**
- **Node.js 18+**
- **Wails CLI** — `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **PortAudio** — `brew install portaudio`
- **Gemini API Key** — [Get one free](https://makersuite.google.com/app/apikey)

## Quick Start

```bash
# Clone the repo
git clone https://github.com/divyadityabhardwaj/voxflow.git
cd voxflow/voxflow-go-implementation

# Install frontend dependencies
cd frontend && npm install && cd ..

# Run in dev mode
wails dev
```

On first launch, the app will download the Whisper model (~142MB).

## Building for Production

```bash
wails build
```

The `.app` bundle will be in `build/bin/`.

## Configuration

Settings are stored in `~/.voxflow/config.json`:

- **API Key** — Your Gemini API key
- **Hotkey** — Customize the global shortcut
- **Model** — Choose tiny/base/small/medium
- **Mode** — Casual or Formal refinement style

## Tech Stack

| Component | Technology                    |
| --------- | ----------------------------- |
| Framework | Wails v2 (Go + Web)           |
| Frontend  | React + TypeScript + Tailwind |
| STT       | whisper.cpp (local)           |
| LLM       | Gemini 1.5 Flash              |
| Database  | SQLite                        |

## License

MIT
