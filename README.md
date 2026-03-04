# VoxFlow

**AI-powered voice dictation for macOS** — Speak naturally, get polished text instantly.

VoxFlow is designed to bridge the gap between spoken thought and written text. By leveraging local AI for transcription and large language models for refinement, it provides a seamless, "it just works" experience for productivity and development.

This project was born from the need to reduce the friction of typing during intensive prompting and coding sessions. It is built for **power users who want to save time** by making speaking faster and more natural, while VoxFlow ensures the output is precise and polished.

> [!IMPORTANT]
> This implementation is currently fine-tuned and optimized specifically for **macOS**.

## Core Vision

### 1. Smart Voice-to-Text

Unlike standard dictation which requires explicit punctuation commands, VoxFlow uses AI to:

- **Auto-Punctuate**: Detects pauses and tone to add commas, periods, and question marks naturally.
- **Clean Up Fillers**: Automatically removes "ums," "uhs," and stutters.
- **Handle "Backtracking"**: Intelligently corrects speech when you change your mind mid-sentence.
- **Understand Context**: Recognizes technical jargon, developer-specific syntax (camelCase), and personal names.

### 2. Universal Integration

VoxFlow acts as a system-level overlay. It works inside any application where you can type:

- **Messaging**: Slack, WhatsApp, iMessage.
- **Productivity**: Notion, Google Docs, Email clients.
- **Development**: VS Code, Cursor, terminal environments.

### 3. Key Productivity Features

- **Styles & Tones**: Transform speech into specific formats (formal email, casual text, or structured lists).
- **History Vault**: Access and search through past transcriptions with raw and polished views.
- **Whisper Mode**: Highly sensitive local transcription that picks up quiet whispering for shared spaces.

## Troubleshooting

### "VoxFlow is damaged and can't be opened"

Because the app is not signed with an Apple Developer Certificate, macOS may block it. To fix this, run:

```bash
xattr -cr /Applications/voxflow.app
```

(Adjust the path if you've moved the app elsewhere)

## Implementations

This repository contains the primary Go-based implementation of VoxFlow.

- **[VoxFlow Go Implementation](./voxflow-go-implementation/README.md)**: The core desktop application built with Wails (Go + React). It handles the global shortcut, local transcription via whisper.cpp, and AI refinement via Gemini, Groq, and other LLMs.
