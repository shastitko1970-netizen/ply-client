# Ply

VPN. Вставила ключ — туннель встал. Не Happ. Не v2rayN.

Два процесса: **окно** и **ядро**. Окно — тот же HTML, что сайт. Крестик закрывает окно — **PlyCore** остаётся и держит туннель.

Понимает **Paper**, **vless://**, **hy2://** / **hysteria2://**, **vmess://**, **trojan://**, **ss://** и https-подписку. Mux выключен. Сайты РФ можно пустить напрямую.

TUIC Xray не умеет. Go качать не надо.

## Скачать

- Windows **1.10.7**: [Ply-1.10.7-win64.zip](https://github.com/shastitko1970-netizen/ply-client/releases/tag/v1.10.7) — запусти **PlySetup.exe**
- Linux **1.10.8** (тест): [Ply-1.10.8-linux-amd64.tar.gz](https://github.com/shastitko1970-netizen/ply-client/releases/tag/v1.10.8) — `./install.sh`
- macOS **1.10.9** (бета): [Ply-1.10.9-macos.zip](https://github.com/shastitko1970-netizen/ply-client/releases/tag/v1.10.9)
- Android **1.10.10** (бета): [Ply-1.10.10.apk](https://github.com/shastitko1970-netizen/ply-client/releases/tag/v1.10.10)

## 1.10.7

- Компактное окно: ключ и тумблеры в одном блоке, лишнее спрятано в «подробнее»
- «РФ напрямую» вместо кривого «Россия мимо»
- Установщик по-прежнему без Go

## 1.10.8

- Linux: окно + ядро + xray, `install.sh` ставит Chromium и setcap

## 1.10.9

- macOS: Ply.app, та же логика

## 1.10.10

- Android: VpnService, ключ, РФ напрямую

## 1.10.6

- Туннель быстрее: MTU 1400, без лишнего DNS, Wi‑Fi больше не душат
