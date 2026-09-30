# Ply

VPN-клиент. Вставила ключ — туннель встал.

Одно ядро в памяти — [Xray](https://github.com/XTLS/Xray-core). Второе не качаем и не держим рядом.
Ключи: `vless://`, `hy2://`, `vmess://`, `trojan://`, `ss://` и https-подписка.
Подписка разбирается в список узлов, запоминается выбранный. Mux выключен. Сайты РФ можно пустить напрямую.

Окно — один HTML. На Windows его рисует WebView2, если его нет — тот же адрес в браузере. Gio остаётся только если не открылось ни то, ни другое.

## Кому это

- Windows 10/11 x64, если нужен tun, а не системный список «VPN».
- Linux и macOS: окно через Chrome, Chromium или Edge.
- Свой ключ или подписка в стандартных схемах.

Не умеет: TUIC, Hysteria v1, WireGuard. Это не Clash с профилями.

## Скачать

Последний релиз: <https://github.com/shastitko1970-netizen/ply-client/releases/latest>

| Платформа | Файл | Как |
|---|---|---|
| Windows | `Ply-*-win64.zip` | `PlySetup.exe`. Go не нужен. |
| Windows portable | `Ply-portable-*-win64.zip` | Папка, рядом `xray.exe` и `wintun.dll`. |
| Linux | `Ply-*-linux-amd64.tar.gz` | `./install.sh` |
| macOS | `Ply-*-macos.zip` | `install.command` → `/Applications/Ply.app`. Нужен Chrome или Edge |

Android: [`android/`](android) и **Ply-2.1.3.apk** в прошлом релизе. На телефоне можно проверить ключ, узлы и приложения мимо VPN. CI сам APK не собирает. В 2.1.4 поправлен обход программ на компьютере; новый APK к этому тегу не приложен.

Крестик закрывает окно. Туннель живёт в ядре. Список VPN Windows — не наш.

Сборки не подписаны. SmartScreen: Подробнее → Выполнить в любом случае.

## Как пользоваться

1. Выключи другие VPN-клиенты.
2. Вставь ключ или ссылку подписки. «Список» покажет узлы, если их несколько. «Обновить подписку» забирает свежий список по той же ссылке. Если сервис прислал новый адрес (`new-url`, `new-domain` или запасной `fallback-url`, как у Happ), старая ссылка сама сменится.
3. «РФ напрямую» — Яндекс, VK, `.ru` без туннеля. Остальное — через узел.
4. «Не выпускать» — kill-switch. На Windows чужой исходящий режется, пока туннель должен жить (наружу ходит только `xray.exe`). На Linux то же через iptables. На macOS тумблер честно скажет, что его ещё нет.
5. «Диагноз» — хвост `xray.log` и кнопка скопировать.
6. «Вместе с системой» поднимает ядро без окна.

## Как устроено

```
Ply.exe        окно
PlyCore.exe    локальный HTTP на 127.0.0.1, держит сессию
xray           tun ply0, mixed 127.0.0.1:10808
```

Чужие имена резолвятся в пул FakeDNS `198.18.0.0/16`. Имя на сервер уходит из TLS/HTTP, не адрес провайдера. IPv4 первым. IPv6 в tun не кладём.

Ключ на диске — файл `url.txt` с правами `0600`. На Windows он ещё закрыт DPAPI. `config.json` Xray читает открытым текстом, поэтому он тоже `0600` и удаляется, когда туннель гаснет.

## Собрать

Нужен Go 1.26.

```bash
go test ./internal/core/...
go build -trimpath -ldflags "-s -w -X ply/internal/core.Version=2.1.4" -o PlyCore ./cmd/worker
```

Windows-окно: `./cmd/ply`. Linux и macOS-окно: `./cmd/launch` (это бинарь `Ply` в архиве).
Рядом: `xray`, на Windows ещё `wintun.dll`, `geoip.dat`, `geosite.dat`. Либо `PLY_XRAY`.

Релиз собирает [`.github/workflows/release.yml`](.github/workflows/release.yml) по тегу `v*`. Версия в бинаре берётся из тега, не из зашитой строки в yaml.

## Журнал

Коротко — в [CHANGELOG.md](CHANGELOG.md).

## Лицензия

MIT. Xray, geoip/geosite и WinTun — их лицензии, не наши.
