#!/bin/sh
set -e

REPO="vaibhav1805/semanticmesh"
BINARY="semanticmesh"
INSTALL_DIR="/usr/local/bin"

# Get latest release tag
LATEST_TAG=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
    echo "Error: could not determine latest release"
    exit 1
fi

echo "Installing ${BINARY} ${LATEST_TAG}..."

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case "$OS" in
    darwin|linux) ;;
    *)            echo "Unsupported OS: $OS"; exit 1 ;;
esac

FILENAME="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${FILENAME}"

echo "Downloading ${URL}..."

# Download to temp file
TMP=$(mktemp)
if ! curl -sL -o "$TMP" "$URL"; then
    echo "Error: download failed"
    rm -f "$TMP"
    exit 1
fi

# Check if we got a valid binary (not a 404 HTML page)
if file "$TMP" | grep -q "text"; then
    echo "Error: binary not found for ${OS}-${ARCH} at ${LATEST_TAG}"
    rm -f "$TMP"
    exit 1
fi

chmod +x "$TMP"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP" "${INSTALL_DIR}/${BINARY}"
else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "$TMP" "${INSTALL_DIR}/${BINARY}"
fi

echo "Installed ${BINARY} ${LATEST_TAG} to ${INSTALL_DIR}/${BINARY}"
${BINARY} --help 2>/dev/null | head -1 || true
