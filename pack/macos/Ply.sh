#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
ARCH="$(uname -m)"
case "$ARCH" in
  arm64) BIN="$DIR/bin/arm64" ;;
  *)     BIN="$DIR/bin/amd64" ;;
esac
if [ ! -x "$BIN/Ply" ]; then
  if [ -x "$DIR/bin/arm64/Ply" ]; then BIN="$DIR/bin/arm64"
  elif [ -x "$DIR/bin/amd64/Ply" ]; then BIN="$DIR/bin/amd64"
  fi
fi
exec "$BIN/Ply" "$@"
