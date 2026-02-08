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

### Creating a DMG (Optional)

To create a distributable `.dmg` file locally (like the GitHub Release does), you need `create-dmg`:

```bash
brew install create-dmg
```

Then run:

```bash
create-dmg \
  --volname "Voxflow Installer" \
  --volicon "build/appicon.png" \
  --window-pos 200 120 \
  --window-size 800 400 \
  --icon-size 100 \
  --icon "voxflow.app" 200 190 \
  --hide-extension "voxflow.app" \
  --app-drop-link 600 185 \
  "build/bin/voxflow.dmg" \
  "build/bin/voxflow.app"
```

### Troubleshooting

**"Voxflow is damaged and can't be opened"**
This happens because the app is not signed with an Apple Developer Certificate (which costs $99/year). To fix it, run this command in your terminal:

```bash
xattr -cr /Applications/voxflow.app
```

(Or point to wherever you unknowingly dragged the app)

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
