#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
DEST="${PLY_DEST:-/opt/ply}"

need() { command -v "$1" >/dev/null 2>&1; }

install_pkgs() {
  if need apt-get; then
    sudo apt-get update -y >/dev/null 2>&1 || true
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y "$@" >/dev/null 2>&1 || true
  elif need dnf; then
    sudo dnf install -y "$@" >/dev/null 2>&1 || true
  elif need pacman; then
    sudo pacman -S --noconfirm "$@" >/dev/null 2>&1 || true
  elif need zypper; then
    sudo zypper --non-interactive install "$@" >/dev/null 2>&1 || true
  fi
}

echo "Ply Linux — ставлю в $DEST"
if ! need sudo; then
  echo "нужен sudo" >&2
  exit 1
fi

install_pkgs libcap2-bin libcap iproute2
if ! need google-chrome && ! need google-chrome-stable && ! need chromium && ! need chromium-browser && ! need microsoft-edge && ! need brave-browser; then
  echo "ставлю Chromium — без него окно не откроется"
  install_pkgs chromium chromium-browser google-chrome-stable
fi

sudo mkdir -p "$DEST"
sudo cp -f "$ROOT/Ply" "$ROOT/PlyCore" "$ROOT/xray" "$DEST/"
sudo cp -f "$ROOT/geoip.dat" "$ROOT/geosite.dat" "$DEST/" 2>/dev/null || true
if [ -f "$ROOT/icon.png" ]; then
  sudo cp -f "$ROOT/icon.png" "$DEST/icon.png"
fi
sudo chmod 755 "$DEST/Ply" "$DEST/PlyCore" "$DEST/xray"

if need setcap; then
  sudo setcap cap_net_admin,cap_net_raw+ep "$DEST/PlyCore" || true
  sudo setcap cap_net_admin,cap_net_raw+ep "$DEST/xray" || true
else
  echo "setcap нет — туннель попросит root"
  install_pkgs libcap2-bin
  if need setcap; then
    sudo setcap cap_net_admin,cap_net_raw+ep "$DEST/PlyCore" "$DEST/xray" || true
  fi
fi

APPDIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
mkdir -p "$APPDIR" "$HOME/.local/bin"
ICON="$DEST/icon.png"
[ -f "$ICON" ] || ICON="$DEST/Ply"
cat > "$APPDIR/ply.desktop" <<EOF
[Desktop Entry]
Name=Ply
Comment=VPN. Paper, vless, hy2, vmess, trojan, ss.
Exec=$DEST/Ply
Icon=$ICON
Terminal=false
Type=Application
Categories=Network;
StartupWMClass=Ply
EOF
chmod 644 "$APPDIR/ply.desktop"
ln -sf "$DEST/Ply" "$HOME/.local/bin/ply"
if need update-desktop-database; then
  update-desktop-database "$APPDIR" >/dev/null 2>&1 || true
fi
echo "Готово. Запуск: $DEST/Ply  или команда ply"
echo "Окно — Chrome/Chromium в режиме приложения. Ядро PlyCore живёт отдельно."
echo "Если туннель не встаёт без пароля — ещё раз: sudo setcap cap_net_admin,cap_net_raw+ep $DEST/PlyCore $DEST/xray"
