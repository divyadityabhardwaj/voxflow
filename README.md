# VoxFlow

macOS voice dictation that turns speech into polished text — locally transcribed, optionally refined by an LLM, then injected into whatever app you're using.

Built for developers and power users who want speaking to be faster than typing without sacrificing quality.

> Optimized for macOS (Apple Silicon).

## Features

- **Local speech-to-text** — [whisper.cpp](https://github.com/ggml-org/whisper.cpp) runs on-device; audio never leaves your machine for transcription
- **Streaming transcription** — chunks while you speak, cut at pauses so words aren't split mid-utterance
- **Resident model** — `whisper-server` keeps the model loaded; no cold start per chunk
- **LLM refinement** — Gemini, OpenRouter, Groq, Cerebras, or any local OpenAI-compatible server (Ollama, LM Studio, llama.cpp)
- **Pipeline modes** — refine, raw, or copy-only
- **Per-app rules** — override mode and delivery (paste / typed keystrokes / clipboard) per application
- **Global hotkeys** — hands-free toggle and push-to-talk, plus a menu bar control
- **Custom vocabulary** — names and jargon fed to Whisper and the refinement prompt
- **History** — search past transcripts, compare raw vs polished, rewrite with a custom instruction

## How it works

1. Press a hotkey (or use the menu bar) to start recording
2. PortAudio captures 16 kHz audio; chunks stream to Whisper
3. Raw text is optionally refined by your chosen LLM
4. Polished text is pasted (or typed / copied) into the frontmost app via CoreGraphics

## Tech stack

| Layer | Choice |
|-------|--------|
| Desktop shell | [Wails v2](https://wails.io/) (Go + WebView) |
| Backend | Go |
| Frontend | React, TypeScript, Tailwind CSS |
| STT | whisper.cpp (`whisper-server` / `whisper-cli`) |
| Refinement | Gemini · OpenRouter · Groq · Cerebras · local OpenAI-compatible |
| Storage | SQLite (`~/.voxflow/history.db`) |

## Prerequisites

- macOS (Apple Silicon recommended)
- Go 1.24+
- Node.js 20.19+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation): `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- PortAudio: `brew install portaudio`
- whisper.cpp: `brew install whisper-cpp`

## Development

```bash
./dev.sh
```

On first launch the app helps download a Whisper model (~142 MB for `base`).

## Configuration

Settings live in `~/.voxflow/config.json`. Models are stored under `~/.voxflow/models/`.

| Setting | Purpose |
|---------|---------|
| LLM provider & model | Cloud or local refinement |
| Pipeline mode | Refine / raw / copy-only |
| Hotkeys | Hands-free and push-to-talk |
| Whisper model & language | Speed vs accuracy; fixed language or auto-detect |
| Vocabulary | Preferred spellings for Whisper + LLM |
| Per-app rules | Mode and inject method per bundle ID |
| Mute system audio | Silence speakers while recording |

Copy `.env.example` to `.env` for API keys during development (keys can also be set in the Settings UI).

## Project layout

```text
.
├── main.go, app*.go     # Wails entry + frontend-bound methods
├── internal/
│   ├── orchestrator/    # Record → transcribe → refine → inject
│   ├── audio/           # PortAudio capture, chunking, mute
│   ├── whisper/         # Models, whisper-server, CLI fallback
│   ├── llm/             # Shared prompt, parsing, OpenAI client
│   ├── gemini/, groq/, cerebras/, openrouter/, localclient/
│   ├── injection/       # Paste / type / clipboard (CoreGraphics)
│   ├── hotkey/, window/, history/, config/, macos/
│   └── logger/, events/
├── frontend/            # React + TypeScript UI
└── docs/                # Product & architecture notes
```

## Troubleshooting

### "VoxFlow is damaged and can't be opened"

The app is not notarized. Clear the quarantine flag:

```bash
xattr -cr /Applications/voxflow.app
```

## Docs

- [Product overview](./docs/PRODUCT.md) — architecture, pipeline, and design decisions in detail

## License

Personal project. See repository for terms.
