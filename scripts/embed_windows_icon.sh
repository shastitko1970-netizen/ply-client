#!/usr/bin/env bash
# Embed assets/icon.ico into the Windows exe packages. Go links any
# *.syso sitting next to the package. Without these files the exe, the
# installer, shortcuts, and the tray have no icon.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
export PATH="$(go env GOPATH)/bin:${PATH}"
if ! command -v rsrc >/dev/null 2>&1; then
  go install github.com/akavel/rsrc@latest
fi
test -s assets/icon.ico
mkdir -p cmd/worker
rsrc -manifest assets/ui.manifest -ico assets/icon.ico -arch amd64 -o cmd/ply/rsrc_windows.syso
rsrc -manifest assets/app.manifest -ico assets/icon.ico -arch amd64 -o cmd/setup/rsrc_windows.syso
rsrc -manifest assets/app.manifest -ico assets/icon.ico -arch amd64 -o cmd/worker/rsrc_windows.syso
test -s cmd/ply/rsrc_windows.syso
test -s cmd/setup/rsrc_windows.syso
test -s cmd/worker/rsrc_windows.syso
