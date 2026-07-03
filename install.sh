#!/bin/sh
# dx installer — downloads the latest release binary for this platform.
#   curl -fsSL https://raw.githubusercontent.com/agarichan/dx/main/install.sh | sh
# Env: DX_INSTALL_DIR (default ~/.local/bin)
set -eu

REPO="agarichan/dx"
DIR="${DX_INSTALL_DIR:-$HOME/.local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *)
    echo "unsupported architecture: $arch" >&2
    exit 1
    ;;
esac
case "$os" in
  darwin | linux) ;;
  *)
    echo "unsupported OS: $os" >&2
    exit 1
    ;;
esac

asset="dx-$os-$arch"
base="https://github.com/$REPO/releases/latest/download"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading $asset ..." >&2
curl -fsSL "$base/$asset" -o "$tmp/dx"
curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"

want=$(awk -v a="$asset" '$2 == a { print $1 }' "$tmp/SHA256SUMS")
if [ -z "$want" ]; then
  echo "no SHA256SUMS entry for $asset" >&2
  exit 1
fi
if command -v sha256sum > /dev/null 2>&1; then
  got=$(sha256sum "$tmp/dx" | awk '{print $1}')
else
  got=$(shasum -a 256 "$tmp/dx" | awk '{print $1}')
fi
if [ "$want" != "$got" ]; then
  echo "checksum mismatch: want $want got $got" >&2
  exit 1
fi

mkdir -p "$DIR"
install -m 0755 "$tmp/dx" "$DIR/dx"
echo "installed dx $("$DIR/dx" version | awk '{print $2}') -> $DIR/dx"

case ":$PATH:" in
  *":$DIR:"*) ;;
  *) echo "note: $DIR is not on your PATH" >&2 ;;
esac
