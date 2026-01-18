#!/bin/sh
#
# Installation script for gh-inspect
#
# Usage: curl -sfL https://raw.githubusercontent.com/mikematt33/gh-inspect/main/install.sh | sh
#

set -e

OWNER="mikematt33"
REPO="gh-inspect"
BINARY="gh-inspect"
FORMAT="tar.gz"
BINDIR=${BINDIR:-"/usr/local/bin"}

# Detect OS and Arch
OS=$(uname -s)
ARCH=$(uname -m)

case $OS in
  Linux) OS="Linux" ;;
  Darwin) OS="Darwin" ;;
  *) echo "OS $OS is not supported"; exit 1 ;;
esac

case $ARCH in
  x86_64) ARCH="x86_64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  i386) ARCH="i386" ;;
  *) echo "Architecture $ARCH is not supported"; exit 1 ;;
esac

# Check for sudo if not root and trying to install to /usr/local/bin
SUDO=""
if [ "$(id -u)" != "0" ] && [ "$BINDIR" = "/usr/local/bin" ]; then
    SUDO="sudo"
fi

# Determine latest version
if [ -z "$VERSION" ]; then
    echo "Finding latest version..."
    LATEST_URL="https://api.github.com/repos/$OWNER/$REPO/releases/latest"
    if [ -n "$GITHUB_TOKEN" ]; then
        VERSION=$(curl -s -H "Authorization: token $GITHUB_TOKEN" $LATEST_URL | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        VERSION=$(curl -s $LATEST_URL | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    fi
fi

if [ -z "$VERSION" ]; then
    echo "Error: Could not find latest version."
    exit 1
fi

echo "Downloading version $VERSION..."

# Construct naming convention matching .goreleaser.yaml
# Format: gh-inspect_Linux_x86_64.tar.gz
ASSET_NAME="${BINARY}_${OS}_${ARCH}.${FORMAT}"
if [ -z "$URL" ]; then
    URL="https://github.com/$OWNER/$REPO/releases/download/$VERSION/$ASSET_NAME"
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading $URL..."
if ! curl -sfL "$URL" -o "$TMP_DIR/$ASSET_NAME"; then
    echo "Error downloading $URL"
    exit 1
fi

echo "Installing..."
cd "$TMP_DIR"
tar -xzf "$ASSET_NAME"

if [ -f "$BINARY" ]; then
    echo "Promoting binary to $BINDIR..."
    $SUDO mv "$BINARY" "$BINDIR/$BINARY"
    $SUDO chmod +x "$BINDIR/$BINARY"
    echo "Successfully installed $BINARY to $BINDIR"
    echo "Run '$BINARY --help' to get started."
else
    echo "Error: Binary not found in archive."
    exit 1
fi
