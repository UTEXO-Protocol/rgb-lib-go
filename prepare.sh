#!/usr/bin/env bash

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[1;36m'
NC='\033[0m' # No color

echo -e "${CYAN}[prepare.sh] Checking system configuration...${NC}"

# Determine OS and target library
OS=$(uname -s)
ARCH=$(uname -m)
TARGET_LIB=""

if [ "$OS" = "Darwin" ]; then
    TARGET_LIB="librgblibuniffi.dylib"
elif [ "$OS" = "Linux" ]; then
    TARGET_LIB="librgblibuniffi.so"
else
    echo -e "${RED}[prepare.sh] Unsupported OS: $OS${NC}"
    exit 1
fi

echo -e "${GREEN}[prepare.sh] Detected OS: $OS ($ARCH)${NC}"
echo -e "${GREEN}[prepare.sh] Targeting: $TARGET_LIB${NC}"

# Create ./lib if missing
mkdir -p ./lib

# Locate Go module directory
echo -e "${CYAN}[prepare.sh] Locating Go module path...${NC}"
PKG_PATH=$(go list -f '{{.Dir}}' -m github.com/UTEXO-Protocol/rgb-lib-go 2>/dev/null)

if [ -z "$PKG_PATH" ]; then
    echo -e "${RED}[prepare.sh] Error: Could not find module github.com/UTEXO-Protocol/rgb-lib-go${NC}"
    exit 1
fi

# Check and copy dynamic lib
SOURCE_LIB="$PKG_PATH/lib/$TARGET_LIB"

if [ ! -f "$SOURCE_LIB" ]; then
    echo -e "${RED}[prepare.sh] Error: Library not found at $SOURCE_LIB${NC}"
    exit 1
fi

cp "$SOURCE_LIB" ./lib/
echo -e "${GREEN}[prepare.sh] Successfully copied $TARGET_LIB to ./lib${NC}"
