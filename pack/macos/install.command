#!/bin/bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
APP="/Applications/Ply.app"
echo "Ply macOS — копирую в $APP"
rm -rf "$APP"
cp -R "$ROOT/Ply.app" "$APP"
xattr -dr com.apple.quarantine "$APP" 2>/dev/null || true
chmod -R u+w "$APP"
find "$APP/Contents/MacOS" -type f -exec chmod 755 {} \;
echo "Готово. Открой Ply из Программ."
echo "Если Gatekeeper ругается: ПКМ по Ply → Открыть."
open "$APP"
