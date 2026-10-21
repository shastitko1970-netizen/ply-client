#!/usr/bin/env bash
# Cross-build the Windows exes and refuse a binary with no icon resource.
# Also builds one exe before embedding, so a checker that accepts everything
# fails this script.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
py=python3
command -v python3 >/dev/null 2>&1 || py=python
out="$(mktemp -d)"
cleanup() {
  rm -rf "$out"
  rm -f cmd/setup/payload.zip
}
trap cleanup EXIT
rm -f cmd/ply/rsrc_windows.syso cmd/setup/rsrc_windows.syso cmd/worker/rsrc_windows.syso
# GOOS stays on the host here. rsrc is a host tool; a Windows GOOS would
# install a Windows rsrc binary and the embed step could not run it.
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H windowsgui" -o "$out/bare.exe" ./cmd/ply
if "$py" scripts/check_pe_icon.py "$out/bare.exe"; then
  echo "checker accepted an exe with no icon resource" >&2
  exit 1
fi
bash scripts/embed_windows_icon.sh
"$py" - <<'PY'
import zipfile
with zipfile.ZipFile("cmd/setup/payload.zip", "w") as z:
    z.writestr("Ply.exe", b"ply")
PY
ldflags='-s -w -H windowsgui -X ply/internal/core.Version=iconcheck'
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o "$out/Ply.exe" ./cmd/ply
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o "$out/PlyCore.exe" ./cmd/worker
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$ldflags" -o "$out/PlySetup.exe" ./cmd/setup
"$py" scripts/check_pe_icon.py "$out/Ply.exe" "$out/PlyCore.exe" "$out/PlySetup.exe"
