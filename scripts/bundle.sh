#!/bin/bash
# Copies the whisper binaries into a built app, ad-hoc signs it, and fails if
# anything still links a Homebrew library (the app must run without Homebrew).
#
# Usage: scripts/bundle.sh <path/to/voxflow.app> <deps prefix>
set -euo pipefail

APP="${1:?usage: $0 <app> <deps prefix>}"
PREFIX="${2:?usage: $0 <app> <deps prefix>}"
MACOS="$APP/Contents/MacOS"
MAIN="$MACOS/$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$APP/Contents/Info.plist")"

cp "$PREFIX/bin/whisper-cli" "$PREFIX/bin/whisper-server" "$MACOS/"
codesign --force --deep --sign - "$APP"
codesign --verify --deep --strict "$APP"

status=0
for bin in "$MAIN" "$MACOS/whisper-cli" "$MACOS/whisper-server"; do
	otool -L "$bin"
	# The bundle ships no dylibs, so any @rpath reference is unresolvable too.
	if otool -L "$bin" | grep -E '/opt/homebrew|/usr/local|@rpath'; then
		echo "error: $bin links a library that won't exist on a user's Mac" >&2
		status=1
	fi
done
exit $status
