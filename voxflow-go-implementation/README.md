# VoxFlow (Go Implementation)

**The core desktop application for VoxFlow, built with Go, Wails, and React.**

VoxFlow is a macOS-optimized voice-to-text tool that captures your voice, transcribes it locally, and uses LLMs to refine it into professional, ready-to-use text.

## How It Works

1.  **Global Capture**: A system-wide hotkey (`Cmd+Shift+V`) triggers the recording state.
2.  **Audio Processing**: The app captures system audio via PortAudio and saves it as a high-fidelity WAV file.
3.  **Local Transcription**: The audio is processed locally using `whisper.cpp`. No voice data leaves your machine.
4.  **AI refinement**: The raw text is sent to an LLM (Gemini, Groq, OpenRouter, or Local GGUF) for punctuation, filler removal, and style formatting.
5.  **Smart Injection**: The polished text is automatically injected into the active application's cursor position using AppleScript.

## Project Structure

```text
.
├── app.go                # Main Wails application logic & event handling
├── main.go               # Entry point and dependency injection
├── window_darwin.go      # macOS-specific window styling (rounded corners, etc.)
├── internal/             # Core backend services
│   ├── audio/            # PortAudio recording implementation
│   ├── hotkey/           # Global shortcut management (macOS)
│   ├── whisper/          # whisper.cpp CLI wrapper and model management
│   ├── llm/              # Refinement prompts and provider abstractions
│   ├── injection/        # AppleScript-based text injection service
│   ├── history/          # SQLite-based storage for past transcripts
│   └── config/           # App settings and API key management
└── frontend/             # Single Page Application (React + Vite)
    ├── src/
    │   ├── components/   # MainView, HistoryView, Settings, etc.
    │   └── contexts/     # Application state management
```

## Tech Stack

- **Framework**: [Wails v2](https://wails.io/) (Go backend, Web frontend)
- **Frontend**: React + TypeScript + Tailwind CSS
- **STT Engine**: [whisper.cpp](https://github.com/ggerganov/whisper.cpp) (Local, high-performance C++)
- **Refinement**: Google Gemini, Groq, OpenRouter, Cerebras, or local LLMs (Ollama/GGUF)
- **Database**: SQLite (via `modernc.org/sqlite`)
- **Injection**: AppleScript (for seamless system-level pasting)

## Prerequisites

- **macOS** (Optimized for Apple Silicon)
- **Go 1.21+**
- **Node.js 18+**
- **Wails CLI**: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **PortAudio**: `brew install portaudio`
- **whisper-cli**: `brew install whisper-cpp`

## Development

For a simplified setup and launch, use the provided development script:

```bash
chmod +x dev.sh
./dev.sh
```

On first launch, the app will assist in downloading the necessary Whisper models (~142MB for base).

## Configuration

Settings are stored in `~/.voxflow/config.json`. You can configure:

- **LLM Provider**: Choose between Gemini, OpenRouter, or Local.
- **Refinement Mode**: "Casual" for messages or "Formal" for documents.
- **Global Hotkey**: Customize the trigger shortcut.
- **Whisper Model**: Balance between speed (`tiny`) and accuracy (`medium`).

---

_VoxFlow is a personal project built for speed and privacy. It's designed to make voice the primary input method for developers and power users._
