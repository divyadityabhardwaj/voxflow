#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🎙️  Voxflow Dev Launcher${NC}"
echo ""

# Detect where we're being run from
# If we're in the frontend directory, go to parent
if [ "$(basename "$(pwd)")" = "frontend" ]; then
    cd ..
    echo -e "${BLUE}📁 Running from frontend/, moved to root${NC}"
fi

# version_ge HAVE NEED
version_ge() {
    [ "$(printf '%s\n' "$2" "$1" | sort -V | head -1)" = "$2" ]
}

GO_MIN=$(awk '/^go /{print $2}' go.mod)
# Vite 7 needs Node 20.19+
NODE_MIN=20.19.0

if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go is not installed. Please install Go $GO_MIN+ first.${NC}"
    echo "   Visit: https://go.dev/dl/"
    exit 1
fi
GO_HAVE=$(go env GOVERSION | sed 's/^go//')
if ! version_ge "$GO_HAVE" "$GO_MIN"; then
    echo -e "${RED}❌ Go $GO_HAVE is too old; go.mod needs $GO_MIN+.${NC}"
    exit 1
fi

if ! command -v node &> /dev/null || ! command -v npm &> /dev/null; then
    echo -e "${RED}❌ Node.js is not installed. Please install Node.js $NODE_MIN+ first.${NC}"
    echo "   Visit: https://nodejs.org/"
    exit 1
fi
NODE_HAVE=$(node --version | sed 's/^v//')
if ! version_ge "$NODE_HAVE" "$NODE_MIN"; then
    echo -e "${RED}❌ Node.js $NODE_HAVE is too old; Vite needs $NODE_MIN+.${NC}"
    exit 1
fi
echo -e "${GREEN}✅ Go $GO_HAVE, Node.js $NODE_HAVE${NC}"

if ! command -v ollama &> /dev/null; then
    echo -e "${YELLOW}ℹ️  ollama not found (optional, only for local refinement: brew install ollama)${NC}"
fi

# Check if wails is installed
if ! command -v wails &> /dev/null; then
    echo -e "${YELLOW}⚠️  Wails CLI not found. Installing...${NC}"
    
    # Check if GOPATH/bin is in PATH
    GOPATH_BIN=$(go env GOPATH)/bin
    if [[ ":$PATH:" != *":$GOPATH_BIN:"* ]]; then
        echo -e "${YELLOW}⚠️  GOPATH/bin not in PATH. Adding temporarily...${NC}"
        export PATH=$PATH:$GOPATH_BIN
    fi
    
    # Install wails
    echo -e "${BLUE}📦 Installing Wails CLI...${NC}"
    go install "github.com/wailsapp/wails/v2/cmd/wails@$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2)"
    
    if ! command -v wails &> /dev/null; then
        echo -e "${YELLOW}⚠️  Wails not in PATH after install. Trying to use from GOPATH...${NC}"
        export PATH=$PATH:$(go env GOPATH)/bin
    fi
fi

# Verify wails is available
if ! command -v wails &> /dev/null; then
    echo -e "${RED}❌ Failed to install Wails. Please add this to your shell profile:${NC}"
    echo "   export PATH=\"\$PATH:\$(go env GOPATH)/bin\""
    exit 1
fi

WAILS_VERSION=$(wails version 2>/dev/null | head -1 || echo "unknown")
echo -e "${GREEN}✅ Wails CLI: $WAILS_VERSION${NC}"

# Install frontend dependencies if needed
if [ ! -d "frontend/node_modules" ]; then
    echo -e "${BLUE}📦 Installing frontend dependencies...${NC}"
    (cd frontend && npm install)
    if [ $? -ne 0 ]; then
        echo -e "${RED}❌ Failed to install frontend dependencies${NC}"
        exit 1
    fi
    echo -e "${GREEN}✅ Frontend dependencies installed${NC}"
else
    echo -e "${GREEN}✅ Frontend dependencies already installed${NC}"
fi

# Check for PortAudio
if ! brew list portaudio &> /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  PortAudio not found. Installing via Homebrew...${NC}"
    brew install portaudio
    if [ $? -ne 0 ]; then
        echo -e "${RED}❌ Failed to install PortAudio${NC}"
        exit 1
    fi
    echo -e "${GREEN}✅ PortAudio installed${NC}"
else
    echo -e "${GREEN}✅ PortAudio is installed${NC}"
fi

if ! command -v whisper-cli &> /dev/null; then
    echo -e "${YELLOW}⚠️  whisper-cli not found; transcription won't work in dev. Run: brew install whisper-cpp${NC}"
fi

echo ""
echo -e "${BLUE}🚀 Starting Voxflow in dev mode...${NC}"
echo ""

# Run wails dev
wails dev
