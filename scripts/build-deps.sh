#!/bin/bash
# Builds the native dependencies the release bundle needs so the app runs on a
# Mac without Homebrew:
#   <prefix>/lib/libportaudio.a         static only, so cgo links it into the app
#   <prefix>/bin/whisper-{cli,server}   self-contained, Metal shaders embedded
#
# Usage: scripts/build-deps.sh <prefix>
# Then:  PKG_CONFIG_PATH=<prefix>/lib/pkgconfig wails build ...
#        scripts/bundle.sh build/bin/voxflow.app <prefix>
set -euo pipefail

PORTAUDIO_VERSION=19.7.0
WHISPER_VERSION=1.8.4
export MACOSX_DEPLOYMENT_TARGET=11.0

PREFIX="${1:?usage: $0 <prefix>}"
mkdir -p "$PREFIX/bin"
PREFIX="$(cd "$PREFIX" && pwd)"
SRC="$(mktemp -d)"
trap 'rm -rf "$SRC"' EXIT
JOBS="$(sysctl -n hw.ncpu)"

echo "==> PortAudio $PORTAUDIO_VERSION"
curl -fsSL "https://github.com/PortAudio/portaudio/archive/refs/tags/v$PORTAUDIO_VERSION.tar.gz" | tar xz -C "$SRC"
(
	cd "$SRC/portaudio-$PORTAUDIO_VERSION"
	# configure adds -Werror; 19.7.0 predates this clang warning.
	./configure --prefix="$PREFIX" --disable-shared --enable-static --enable-mac-universal=no \
		CFLAGS="-O2 -Wno-implicit-const-int-float-conversion"
	make -j"$JOBS"
	make install
)

echo "==> whisper.cpp $WHISPER_VERSION"
curl -fsSL "https://github.com/ggml-org/whisper.cpp/archive/refs/tags/v$WHISPER_VERSION.tar.gz" | tar xz -C "$SRC"
# GGML_NATIVE=OFF keeps the build machine's CPU extensions out of the binary;
# GGML_OPENMP=OFF stops it picking up Homebrew's libomp.
cmake -S "$SRC/whisper.cpp-$WHISPER_VERSION" -B "$SRC/whisper-build" \
	-DCMAKE_BUILD_TYPE=Release \
	-DCMAKE_OSX_DEPLOYMENT_TARGET="$MACOSX_DEPLOYMENT_TARGET" \
	-DBUILD_SHARED_LIBS=OFF \
	-DGGML_METAL=ON \
	-DGGML_METAL_EMBED_LIBRARY=ON \
	-DGGML_NATIVE=OFF \
	-DGGML_OPENMP=OFF \
	-DWHISPER_BUILD_TESTS=OFF \
	-DWHISPER_BUILD_EXAMPLES=ON
cmake --build "$SRC/whisper-build" --config Release -j "$JOBS" --target whisper-cli whisper-server
cp "$SRC/whisper-build/bin/whisper-cli" "$SRC/whisper-build/bin/whisper-server" "$PREFIX/bin/"

echo "==> Done. Build with PKG_CONFIG_PATH=$PREFIX/lib/pkgconfig"
