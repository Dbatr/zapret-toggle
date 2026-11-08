# Zapret Toggle

Маленькое приложение для Fedora (GNOME): одна кнопка включает и выключает
[zapret](https://github.com/bol-van/zapret) (обход DPI) **только для YouTube и Discord**.
Стратегии — из [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube),
сконвертированные под Linux.

- zapret **не работает постоянно и не стартует с системой** — только по кнопке;
- по умолчанию выключается при закрытии окна (как закрытие батника на Windows);
- root нужен только для установки: дальше start/stop разрешает polkit, без пароля и без sudoers.

## Как это устроено

```
zapret-toggle (Go + Fyne, от пользователя)
   │  systemctl start/stop zapret-dy@<стратегия>.service      ← разрешено polkit-правилом
   ▼
zapret-dy@.service (systemd, root, без [Install])
   ├─ ExecStartPre  /opt/zapret-dy/firewall.sh up <стратегия>  → таблица nftables inet zapretdy
   ├─ ExecStart     /opt/zapret-dy/run.sh <стратегия>          → nfqws --qnum=200 … (сбрасывает root)
   └─ ExecStopPost  /opt/zapret-dy/firewall.sh down            → таблица удаляется всегда
```

nftables отправляет в очередь 200 только первые пакеты соединений на порты стратегии;
nfqws сам решает по спискам доменов (`/opt/zapret-dy/lists/youtube.txt`, `discord.txt`),
применять ли обход. Весь остальной трафик идёт как обычно. Таблица своя — firewalld она не трогает.

Всё системное лежит в `/opt/zapret-dy`, а не в домашнем каталоге: на Fedora `~` имеет права 0700,
а nfqws после старта работает от непривилегированного пользователя и не прочитал бы списки.

## Установка

Зависимости (Go и заголовки для сборки Fyne):

```bash
sudo dnf install golang gcc make python3 nftables \
    mesa-libGL-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel
# по желанию — нативный Wayland (без них окно работает через XWayland):
sudo dnf install wayland-devel libxkbcommon-devel
```

Установка (из каталога проекта, от своего пользователя через sudo):

```bash
sudo scripts/install.sh      # или: make install
```

Скрипт:
- копирует nfqws, скрипты и фейковые пакеты в `/opt/zapret-dy`, конвертирует стратегии Flowseal
  (`system/convert.py`) и проверяет каждую через `nfqws --dry-run` — не прошедшие не попадут в список;
- ставит `/etc/systemd/system/zapret-dy@.service` и `/etc/polkit-1/rules.d/50-zapret-dy.rules`
  (только для твоего пользователя, только start/stop/restart юнитов `zapret-dy@*`, только в активной
  локальной сессии), делает `daemon-reload` и `restorecon` (SELinux);
- собирает приложение и ставит его от твоего имени: `~/.local/bin/zapret-toggle`, ярлык в меню и иконку.

Повторный запуск безопасен (так же обновляются файлы после `git pull`).

## Использование

Запусти **Zapret Toggle** из меню приложений и нажми на круглую кнопку:

| Цвет    | Состояние                       |
|---------|---------------------------------|
| серый   | выключено                       |
| жёлтый  | запуск / остановка              |
| зелёный | включено: YouTube и Discord     |

- **Стратегия** — какой из вариантов Flowseal использовать (`general`, `general (ALT2)`, …).
  Выбор запоминается; если zapret включён, смена стратегии сразу перезапускает его на новой.
- **Выключать при закрытии окна** — по умолчанию включено. Если снять, zapret продолжит работать
  после закрытия окна; при следующем открытии приложение покажет, что он включён.
- **Солнце / луна** в правом верхнем углу — светлая или тёмная тема. Пока её не переключали,
  окно следует теме системы.
- Размер окна меняется как угодно; кнопка со стрелками или **F11** — на весь экран (выход — F11 или Esc).

Настройки: `~/.config/zapret-toggle/config.json`.

Без приложения то же самое делается так:

```bash
systemctl start zapret-dy@general.service   # пароль не спросит (polkit)
systemctl stop  zapret-dy@general.service
cut -f1 /opt/zapret-dy/strategies/index.txt # список стратегий
```

## Как проверить, что работает

1. Включи zapret кнопкой (стратегия `general`), кнопка должна стать зелёной.
2. Открой YouTube (видео должно грузиться без «вечной загрузки») и Discord
   (сайт/приложение, затем голосовой канал — должно подключиться без «RTC Connecting»).
3. Таблица nftables на месте:
   ```bash
   sudo nft list table inet zapretdy
   ```
   Должны быть цепочки `post`, `pre`, `predefrag` с правилами `queue … to 200`.
4. Сервис и nfqws живы, nfqws работает не от root:
   ```bash
   systemctl status zapret-dy@general.service
   ps -o user,pid,args -C nfqws
   journalctl -u zapret-dy@general.service -n 50
   ```
5. Выключи кнопкой — `sudo nft list table inet zapretdy` должен ответить, что таблицы нет,
   а `ps -C nfqws` — ничего не найти.

## Если не работает

- **Попробуй другую стратегию.** Провайдеры режут по-разному; на Windows у Flowseal тоже обычно
  перебирают `general`, `general (ALT)`, `general (ALT2)` … `general (FAKE TLS AUTO)`,
  `general (SIMPLE FAKE)`. После переключения обнови страницу/перезапусти Discord.
- **Ошибка при включении.** Приложение показывает текст ошибки и команду для журнала:
  `journalctl -u zapret-dy@<стратегия> -n 50`. Если nfqws падает сразу после запуска —
  стратегия не подходит или файлы в `/opt/zapret-dy` повреждены: `sudo scripts/install.sh`.
- **«polkit не разрешил операцию»** — не установлено правило или сессия не активная/не локальная
  (например, запуск по SSH). Переустанови: `sudo scripts/install.sh`.
- **YouTube всё равно тормозит.** В стратегиях Flowseal YouTube обрабатывается по TCP (HTTPS),
  а QUIC (UDP 443) — только для доменов Discord. Если провайдер не блокирует QUIC целиком,
  браузер может уходить в него мимо обхода: отключи QUIC/HTTP3
  (Chrome: `chrome://flags/#enable-quic` → Disabled; Firefox: `network.http.http3.enable` = false).
- **Подмена DNS провайдером** — включи DNS-over-HTTPS в браузере.
- **Стратегии устарели** — обнови Flowseal (ниже).
- Конфликт возможен с другим zapret/nfqws, который тоже использует очередь 200 — выключи его.

## Обновление стратегий Flowseal

```bash
scripts/update-flowseal.sh          # или: make update-flowseal
```

Скачивает последний коммит Flowseal, прогоняет `convert.py` и `nfqws --dry-run` по всем стратегиям
во временном каталоге (не прошедшие исключаются с предупреждением; если не прошла ни одна —
`third_party/flowseal` не меняется), обновляет `third_party/flowseal/COMMIT` и
`third_party/README.md`, затем ставит стратегии в `/opt/zapret-dy` (`sudo scripts/install.sh
--strategies-only`, спросит пароль). Флаги: `--force` — даже если коммит тот же,
`--no-install` — только обновить `third_party`.

## Удаление

```bash
sudo scripts/uninstall.sh           # или: make uninstall
sudo scripts/uninstall.sh --purge   # вместе с настройками приложения
```

Останавливает все `zapret-dy@*`, удаляет таблицу nftables, `/opt/zapret-dy`, юнит, polkit-правило,
приложение, ярлык и иконку.

## Безопасность и ограничения

- Никаких `NOPASSWD` в sudoers. polkit разрешает только `start`/`stop`/`restart` юнитов
  `zapret-dy@<[a-z0-9-]+>.service`, только установившему пользователю и только в активной локальной сессии.
- Юнит без секции `[Install]`: `systemctl enable` на нём не сработает, автозапуска нет.
- Сервис с умеренным hardening: `ProtectSystem=full`, `ProtectHome=yes`, `PrivateTmp=yes`,
  `CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW CAP_SETUID CAP_SETGID`. nfqws после старта
  сбрасывает root и оставляет себе только `CAP_NET_ADMIN` и `CAP_NET_RAW`.
- Только zapret1 (`nfqws` v72.13) и стратегии Flowseal; только YouTube и Discord —
  профили Flowseal с `ipset-all` (Cloudflare и т.п.) и GameFilter выброшены конвертером.
- Значка в трее нет (в GNOME трей без расширения не работает).
- Если приложение убить через `kill -9`, «выключить при закрытии» не сработает — выключи
  zapret, открыв приложение снова, или `systemctl stop 'zapret-dy@*'`.

## Разработка

```bash
make build     # build/zapret-toggle (без заголовков Wayland — автоматически с -tags x11)
make run
make check     # gofmt, shellcheck, go vet, go test, сборка
```

В бинарник встроены шрифт Geist (SIL OFL 1.1, `assets/fonts/OFL.txt`)
и иконки Lucide (ISC, `internal/ui/icons/LICENSE`).

Структура и детали — в [CLAUDE.md](CLAUDE.md). Откуда взяты бинарники и стратегии —
в [third_party/README.md](third_party/README.md).
