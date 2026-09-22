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
ARCH="arm64"
if [ "$(uname -m)" = "x86_64" ]; then ARCH="amd64"; fi
CORE="$APP/Contents/MacOS/bin/$ARCH/PlyCore"
mkdir -p "$HOME/Library/LaunchAgents"
PLIST="$HOME/Library/LaunchAgents/land.ply.vpn.plist"
cat > "$PLIST" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>land.ply.vpn</string>
<key>ProgramArguments</key><array><string>${CORE}</string></array>
<key>RunAtLoad</key><true/>
</dict></plist>
EOF
launchctl bootout "gui/$(id -u)" "$PLIST" >/dev/null 2>&1 || true
launchctl bootstrap "gui/$(id -u)" "$PLIST" >/dev/null 2>&1 || true
echo "Готово. Окно — из Программ. Ядро уже в тихом автозапуске."
echo "Если Gatekeeper ругается: ПКМ по Ply → Открыть."
open "$APP"
