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

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go is not installed. Please install Go 1.21+ first.${NC}"
    echo "   Visit: https://go.dev/dl/"
    exit 1
fi

# Check if Node.js is installed
if ! command -v npm &> /dev/null; then
    echo -e "${RED}❌ Node.js is not installed. Please install Node.js 18+ first.${NC}"
    echo "   Visit: https://nodejs.org/"
    exit 1
fi

# Check for Ollama
if ! command -v ollama &> /dev/null; then
    echo -e "${YELLOW}⚠️  ollama not found. Installing via Homebrew...${NC}"
    brew install ollama
    if [ $? -ne 0 ]; then
        echo -e "${RED}❌ Failed to install ollama${NC}"
        exit 1
    fi
    echo -e "${GREEN}✅ ollama installed${NC}"
else
    echo -e "${GREEN}✅ ollama is available${NC}"
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
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    
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

echo ""
echo -e "${BLUE}🚀 Starting Voxflow in dev mode...${NC}"
echo ""

# Run wails dev
wails dev
