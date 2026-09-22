# Ply

Windows-клиент [Paper VPN](https://papervpn.io). Вставила ссылку из кабинета — ключ чистится сам, Mux выключен, поднимается **VPN-туннель** (Wintun), не системный прокси.

Не Happ. Не v2rayN.

## Скачать

Релиз **1.1.0**: [github.com/shastitko1970-netizen/ply-client/releases/tag/v1.1.0](https://github.com/shastitko1970-netizen/ply-client/releases/tag/v1.1.0)

1. Скачай **Ply-1.1.0-win64.zip**, запусти **PlySetup.exe**
2. Если стоит 1.0 — установщик сам найдёт его и обновит в ту же папку. Ссылка Paper останется.
3. SmartScreen: «Подробнее» → «Выполнить в любом случае»
4. Ply появится в меню Пуск (Все приложения) и на рабочем столе
5. Открой Ply → согласись на права администратора → вставь ссылку → **Включить VPN**
6. Happ и приложение Paper выключи. Один слой.

Когда туннель живой, в Windows появляется адаптер **Ply Tunnel**, в окне Ply — «VPN включён» и IP выхода.

## Что внутри 1.1

- TUN через Xray + `wintun.dll`, адаптер называется Ply Tunnel
- Регистрация в «Приложениях» Windows, ярлык в корне меню Пуск (не в папке)
- Обновление 1.0 → 1.1 без потери `data/url.txt`
- Автозапуск — задача с правами администратора, иначе туннель на логине не встанет

## Сборка

Go 1.23+, Windows cross-compile без CGO.

```bash
export CGO_ENABLED=0 GOOS=windows GOARCH=amd64
rsrc -manifest assets/app.manifest -ico assets/icon.ico -arch amd64 -o cmd/ply/rsrc_windows.syso
rsrc -manifest assets/app.manifest -ico assets/icon.ico -arch amd64 -o cmd/setup/rsrc_windows.syso
go build -trimpath -ldflags="-s -w -H windowsgui" -o dist/Ply.exe ./cmd/ply
# payload.zip: Ply.exe + xray.exe + geoip.dat + wintun.dll → cmd/setup/payload.zip
go build -trimpath -ldflags="-s -w -H windowsgui" -o dist/PlySetup.exe ./cmd/setup
```
