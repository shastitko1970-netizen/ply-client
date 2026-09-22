# Ply

Небольшой Windows-клиент под [Paper VPN](https://papervpn.io). Вставила ссылку из кабинета — ключ чистится сам (двойной `#`), Mux выключен, системный прокси, ядро Xray внутри.

Не Happ. Не v2rayN.

## Скачать

Релизы: [github.com/shastitko1970-netizen/ply-client/releases](https://github.com/shastitko1970-netizen/ply-client/releases)

1. Скачай **Ply-1.0.0-win64.zip**
2. Запусти **PlySetup.exe**
3. Если Windows SmartScreen ругнётся — «Подробнее» → «Выполнить в любом случае». Подписи нет, это обычный самосбор.
4. Вставь ссылку Paper. Happ и приложение Paper выключи.

Установщик кладёт файлы в `%LOCALAPPDATA%\Ply`, ставит ярлык на рабочий стол и в меню Пуск.

## Что делает

- Разбирает `https://…azure-api.net/…` или готовый `vless://`
- Режет хвост `#PaperVPN` и второй `#` в ключе (из‑за них v2rayN часто приходил пустой)
- Пишет config.json без Mux, Reality + Vision как в ключе
- Поднимает Xray на `127.0.0.1:10808` и включает системный прокси Windows
- Автозапуск — галка в окне

TUN нет. Один слой.

## Сборка

Нужен Go 1.23+, Windows cross-compile без CGO.

```bash
export CGO_ENABLED=0 GOOS=windows GOARCH=amd64
go build -trimpath -ldflags="-s -w -H windowsgui" -o dist/Ply.exe ./cmd/ply
# затем payload.zip: Ply.exe + xray.exe + geoip.dat → cmd/setup/payload.zip
go build -trimpath -ldflags="-s -w -H windowsgui" -o dist/PlySetup.exe ./cmd/setup
```

GitHub Actions на `main` собирает zip и кладёт его в GitHub Release `v1.0.0`.
