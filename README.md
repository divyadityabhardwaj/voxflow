# VoxFlow

macOS voice dictation that turns speech into polished text — locally transcribed, optionally refined by an LLM, then injected into whatever app you're using.

## Install

**Requirements:** a Mac with Apple Silicon (M1 or later) running macOS 11 Big Sur or newer. Nothing else — no Homebrew, no Terminal setup.

1. Download `voxflow-arm64.dmg` from the [latest release](https://github.com/divyadityabhardwaj/voxflow/releases/latest).
2. Open it and drag **voxflow** into **Applications**.
3. **First open.** VoxFlow isn't notarized by Apple yet, so macOS blocks it the first time. Open it once, dismiss the warning, then go to **System Settings › Privacy & Security** (on macOS 11–12: System Preferences › Security & Privacy › General) and click **Open Anyway**.
   If macOS instead says the app "is damaged and can't be opened", run this once in Terminal and open it again:
   ```bash
   xattr -cr /Applications/voxflow.app
   ```
4. Grant **Microphone** (to hear you) and **Accessibility** (to paste text into other apps) when asked. The in-app setup walks you through both and downloads a speech model (~142 MB for `base`).
5. Press **⌘⇧Space** to start dictating, press it again to stop — the text appears where your cursor is. Hold **⌘⇧P** for push-to-talk instead.

After an update you may need to grant Microphone and Accessibility again, because each build is signed ad hoc.

### Privacy

- Your audio never leaves your Mac: speech is transcribed on-device by whisper.cpp.
- If AI clean-up is on, only the transcribed **text** is sent to the provider you choose (Gemini, OpenRouter, Groq, Cerebras) — or to a local server such as Ollama, in which case nothing leaves your Mac. Turn clean-up off (Raw mode) to keep everything local.
- Free tiers of some providers may use submitted text to train their models. Check your provider's terms, or use a paid key or a local model for sensitive text.
- Settings, API keys and history stay in `~/.voxflow` on your Mac.

### Uninstall

1. Quit VoxFlow from the menu bar.
2. Delete `/Applications/voxflow.app`.
3. Delete your settings, history and models: `rm -rf ~/.voxflow`
4. Optionally remove VoxFlow from **System Settings › Privacy & Security › Microphone / Accessibility**.

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

## Development

Prerequisites:

- macOS on Apple Silicon
- Go 1.24+
- Node.js 20.19+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation) matching `go.mod`: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0`
- PortAudio: `brew install portaudio`
- whisper.cpp: `brew install whisper-cpp` (dev builds use the Homebrew binaries; release builds bundle their own)

```bash
./dev.sh
```

On first launch the app helps download a Whisper model (~142 MB for `base`).

### Release builds

Tagging `v*` runs `.github/workflows/release.yml`, which produces a DMG that runs without Homebrew. To reproduce it locally (needs `cmake`):

```bash
scripts/build-deps.sh ~/.cache/voxflow-deps          # static PortAudio + whisper.cpp
PKG_CONFIG_PATH=~/.cache/voxflow-deps/lib/pkgconfig wails build -platform darwin/arm64 -clean
scripts/bundle.sh build/bin/voxflow.app ~/.cache/voxflow-deps   # bundle whisper, sign, check linkage
```

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
├── scripts/             # Release: build static deps, bundle + sign the app
└── docs/                # Product & architecture notes
```

## Docs

- [Product overview](./docs/PRODUCT.md) — architecture, pipeline, and design decisions in detail

## License

Personal project. See repository for terms.
