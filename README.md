# Zapret Toggle

Маленькое приложение для Linux с systemd: одна кнопка включает и выключает
[zapret](https://github.com/bol-van/zapret) (обход DPI) **только для YouTube и Discord**.
Стратегии — из [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube),
сконвертированные под Linux.

- zapret **не работает постоянно и не стартует с системой** — только по кнопке;
- по умолчанию выключается при закрытии окна (как закрытие батника на Windows);
- root нужен только для установки: дальше start/stop разрешает polkit, без пароля и без sudoers.

## Где работает

- **Дистрибутивы** (x86_64, systemd): Fedora, Debian 12+, Ubuntu 24.04+, Arch / Manjaro / CachyOS,
  openSUSE Tumbleweed и Leap — и их производные. Название дистрибутива установщик не проверяет:
  ему нужны systemd, nftables и polkit с правилами в `rules.d` (polkit 0.106 и новее).
- **Оболочки**: GNOME, KDE Plasma, XFCE, Cinnamon, MATE, LXQt, Budgie, i3, sway — в сессии X11
  или Wayland (нативно либо через XWayland, смотря как собрано — см. «Установка»).
  От GTK и GNOME приложение не зависит: шрифт и иконки встроены, ярлык и настройки лежат по XDG.

Проверялось на Fedora 45 (GNOME) и — установка и сборка в контейнерах — на Debian 13, Ubuntu 24.04,
Arch и openSUSE Tumbleweed. Что не поддерживается — в разделе [«Ограничения»](#ограничения).

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
применять ли обход. Весь остальной трафик идёт как обычно. Таблица своя — правила firewalld, ufw
и других программ она не трогает.

Всё системное лежит в `/opt/zapret-dy`, а не в домашнем каталоге: `~` обычно закрыт для других
пользователей (0700 или 0750), а nfqws после старта работает от непривилегированного пользователя
и не прочитал бы списки.

## Установка

### Зависимости

Нужны systemd, nftables, polkit, python3 и то, чем собирается приложение: Go, компилятор C, make,
pkg-config, заголовки OpenGL и оконной системы.

| Что                              | Fedora                 | Debian / Ubuntu       | Arch         | openSUSE               |
|----------------------------------|------------------------|-----------------------|--------------|------------------------|
| Go ≥ 1.26.10 (или Go ≥ 1.21, см. ниже) | `golang`         | `golang-go`           | `go`         | `go`                   |
| Компилятор C                     | `gcc`                  | `gcc` `libc6-dev`     | `gcc`        | `gcc`                  |
| make, pkg-config                 | `make` `pkgconf-pkg-config` | `make` `pkg-config` | `make` `pkgconf` | `make` `pkgconf-pkg-config` |
| python3, nftables, polkit        | `python3` `nftables` `polkit` | `python3` `nftables` `polkitd` | `python` `nftables` `polkit` | `python3` `nftables` `polkit` |
| OpenGL (libGL)                   | `mesa-libGL-devel`     | `libgl1-mesa-dev`     | `libglvnd`   | `Mesa-libGL-devel`     |
| Заголовки X11                    | `libX11-devel` `libXcursor-devel` `libXrandr-devel` `libXinerama-devel` `libXi-devel` `libXxf86vm-devel` | `libx11-dev` `libxcursor-dev` `libxrandr-dev` `libxinerama-dev` `libxi-dev` `libxxf86vm-dev` | `libx11` `libxcursor` `libxrandr` `libxinerama` `libxi` `libxxf86vm` | как в Fedora |
| Заголовки Wayland (по желанию)   | `wayland-devel` `libxkbcommon-devel` | `libwayland-dev` `libxkbcommon-dev` `libegl-dev` | `wayland` `libxkbcommon` | `wayland-devel` `libxkbcommon-devel` `Mesa-libEGL-devel` |

Одной командой:

```bash
# Fedora
sudo dnf install golang gcc make pkgconf-pkg-config python3 nftables polkit mesa-libGL-devel \
    libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel \
    wayland-devel libxkbcommon-devel

# Debian, Ubuntu
sudo apt install golang-go gcc libc6-dev make pkg-config python3 nftables polkitd libgl1-mesa-dev \
    libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev libxxf86vm-dev \
    libwayland-dev libxkbcommon-dev libegl-dev

# Arch, Manjaro, CachyOS
sudo pacman -S --needed go gcc make pkgconf python nftables polkit libglvnd \
    libx11 libxcursor libxrandr libxinerama libxi libxxf86vm wayland libxkbcommon

# openSUSE
sudo zypper install go gcc make pkgconf-pkg-config python3 nftables polkit Mesa-libGL-devel \
    libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel \
    wayland-devel libxkbcommon-devel Mesa-libEGL-devel
```

**Go.** `go.mod` требует Go 1.26.10. Если в дистрибутиве Go старее, но это хотя бы 1.21, ничего
делать не надо: при сборке он сам скачает нужную версию (`make` выставляет `GOTOOLCHAIN=auto`, нужен
интернет). Go до 1.21 (как в Debian 12) так не умеет — поставь свежий с <https://go.dev/dl/>.

**X11 и Wayland.** Какие заголовки стоят, такое окно и соберётся — `make` определяет это сам:

| Заголовки        | Сборка               | Как работает окно                                         |
|------------------|----------------------|-----------------------------------------------------------|
| X11 и Wayland    | оба бэкенда          | в Wayland-сессии — нативно, в X11 — как обычно            |
| только X11       | `-tags x11`          | в Wayland-сессии — через XWayland                          |
| только Wayland   | `-tags wayland`      | только Wayland, бинарник не зависит от X11-библиотек       |

Проверить всё заранее, ничего не меняя в системе (можно без sudo):

```bash
scripts/install.sh --check
```

Скрипт перечислит, чего не хватает, и напечатает команду установки пакетов для твоего дистрибутива
(сам он пакеты не ставит).

### Установка

Из каталога проекта, от своего пользователя через sudo:

```bash
sudo scripts/install.sh      # или: make install
```

Скрипт:
- проверяет систему: x86_64, systemd, работающий nftables, polkit, зависимости сборки;
- копирует nfqws, скрипты и фейковые пакеты в `/opt/zapret-dy`, конвертирует стратегии Flowseal
  (`system/convert.py`) и проверяет каждую: опции nfqws — по белому списку, файлы — только из
  `/opt/zapret-dy/fake` и `lists`, затем `nfqws --dry-run` — не прошедшие не попадут в список;
- ставит `/etc/systemd/system/zapret-dy@.service` и `/etc/polkit-1/rules.d/50-zapret-dy.rules`
  (только для твоего пользователя, только start/stop/restart юнитов `zapret-dy@*`, только в активной
  локальной сессии) и делает `daemon-reload`;
- там, где включён SELinux (Fedora, RHEL, новые openSUSE), выставляет метки через `restorecon`;
  в Debian, Ubuntu и Arch этот шаг пропускается — `restorecon` там не нужен;
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
  окно следует теме системы: её сообщает XDG-портал (`xdg-desktop-portal` с бэкендом оболочки —
  есть в GNOME, KDE Plasma и большинстве других). Если портала нет (часто в i3 и sway), окно
  тёмное, а тема переключается этой кнопкой вручную и запоминается.
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
- **Оболочка без polkit-агента (i3, sway, свой оконный менеджер).** Агент не нужен: правило
  разрешает операцию без запроса пароля. Нужны сам polkit (служба `polkit.service`) и сессия,
  которую logind считает активной и локальной:
  `loginctl show-session "$XDG_SESSION_ID" -p Active -p Remote` должно показать `Active=yes`
  и `Remote=no`. Если приложение запущено из-под `su`, по SSH или из чужой сессии — будет отказ.
- **Окно не открывается в Wayland-сессии** (в терминале — ошибка GLFW про X11 или `DISPLAY`).
  Приложение собрано только с X11, а XWayland в сессии нет: он не установлен или выключен
  (например, `xwayland disable` в sway). Поставь заголовки Wayland (таблица в «Установке»)
  и переустанови — `sudo scripts/install.sh`, либо включи XWayland. Что сейчас соберётся,
  показывает `scripts/build-deps.sh check`.
- **Ярлыка или иконки нет в меню** — выйди из сессии и войди снова (KDE Plasma: `kbuildsycoca6`).
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

## Безопасность

- Никаких `NOPASSWD` в sudoers. polkit разрешает только `start`/`stop`/`restart` юнитов
  `zapret-dy@<[a-z0-9-]+>.service`, только установившему пользователю и только в активной локальной сессии.
- Юнит без секции `[Install]`: `systemctl enable` на нём не сработает, автозапуска нет.
- Аргументы nfqws из `.bat` Flowseal не доверенные: опции сверяются с белым списком (без `--pidfile`,
  `--debug`, `--uid`/`--user`, `--hostlist-auto`, своих `--qnum` и `--dpi-desync-fwmark`), пути —
  только внутри `/opt/zapret-dy/fake` и `lists`; символические ссылки в Flowseal отвергаются.
- Сервис в песочнице systemd: `CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW CAP_SETUID CAP_SETGID`,
  `NoNewPrivileges`, `ProtectSystem=strict`, `ProtectHome`, `PrivateTmp`, `PrivateDevices`, защита
  ядра (`ProtectKernel*`, `ProtectControlGroups`, `ProtectClock`), фильтр системных вызовов
  `@system-service`, сокеты только `AF_UNIX`/`AF_NETLINK`/`AF_INET`/`AF_INET6`
  (`systemd-analyze security` — 2.6 вместо 7.0). nfqws после старта сбрасывает root и оставляет
  себе только `CAP_NET_ADMIN` и `CAP_NET_RAW`.
- Всё это одинаково на любом дистрибутиве: установщик меняет только то, чего в системе нет
  (шаг `restorecon` без SELinux).

## Ограничения

- **Только systemd.** Дистрибутивы с другим init — Void (runit), Alpine и Artix (OpenRC), Devuan —
  не поддерживаются: zapret запускается юнитом systemd, а включать его разрешает polkit.
- **Только x86_64.** В `third_party` лежит nfqws для x86_64; на ARM (Raspberry Pi, Asahi) нужен
  другой бинарник zapret — установщик остановится с ошибкой.
- **Ядро** должно уметь `nf_tables` и очереди nfqueue (`nft_queue`, `nfnetlink_queue`) — в ядрах
  перечисленных дистрибутивов это есть; в урезанных и самосборных может не быть.
- **polkit 0.106 и новее** (правила на JavaScript в `rules.d`). В Ubuntu 22.04 и Debian 11 polkit
  старее — на них не установится.
- **Чистый Wayland без XWayland** требует сборки с заголовками Wayland (см. «Установка»).
  В сборке «только Wayland» курсор «загрузка» в GNOME после запуска может крутиться до таймаута:
  сообщение о завершении запуска отправляется только через X11.
- **Flatpak, Snap и NixOS** не поддерживаются: установка идёт в `/opt`, `/etc` и `~/.local`,
  а приложению нужен `systemctl` хоста. Неизменяемые системы (Fedora Silverblue, openSUSE
  MicroOS) не проверялись.
- Только zapret1 (`nfqws` v72.13) и стратегии Flowseal; только YouTube и Discord —
  профили Flowseal с `ipset-all` (Cloudflare и т.п.) и GameFilter выброшены конвертером.
- Значка в трее нет (в GNOME трей без расширения не работает).
- Если приложение убить через `kill -9`, «выключить при закрытии» не сработает — выключи
  zapret, открыв приложение снова, или `systemctl stop 'zapret-dy@*'`.

## Разработка

```bash
make build     # build/zapret-toggle; бэкенды окна — по установленным заголовкам (X11, Wayland или оба)
make build TAGS=x11        # явно: только X11; TAGS=wayland — только Wayland; TAGS= — оба
make run
make check     # gofmt, shellcheck, go vet, тесты скриптов и go test, сборка
scripts/build-deps.sh check   # чего не хватает для сборки и какие пакеты поставить
```

В бинарник встроены шрифт Geist (SIL OFL 1.1, `assets/fonts/OFL.txt`)
и иконки Lucide (ISC, `internal/ui/icons/LICENSE`).

Структура и детали — в [CLAUDE.md](CLAUDE.md). Откуда взяты бинарники и стратегии —
в [third_party/README.md](third_party/README.md).
