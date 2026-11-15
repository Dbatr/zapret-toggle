# zapret-toggle — заметки для Claude Code

Десктоп-приложение (Go + Fyne v2.8): одна кнопка включает/выключает zapret1 (nfqws v72.13)
только для YouTube и Discord, со стратегиями Flowseal. Интерфейс, README и сообщения — на русском.

Рассчитано на systemd-дистрибутивы x86_64 (Fedora, Debian/Ubuntu, Arch и производные, openSUSE)
и оболочки GNOME, KDE Plasma, XFCE, Cinnamon, MATE, LXQt, Budgie, i3, sway (X11 и Wayland);
GUI проверен только в GNOME, остальные оболочки требуют ручной проверки.
Основная машина разработки — Fedora/GNOME: поведение и безопасность на ней менять нельзя.

## Архитектура

- `cmd/zapret-toggle` — точка входа; app ID `zapret-toggle` = имя .desktop (Wayland app_id),
  X11 WM_CLASS = `Zapret Toggle` (= `StartupWMClass`). GNOME Shell 51 передаёт DESKTOP_STARTUP_ID
  даже при `StartupNotify=false`, а X11-бэкенд GLFW startup notification не завершает — курсор «загрузка»
  крутился до таймаута. Поэтому `internal/startupnotify` (cgo, libX11) в `OnStarted` шлёт X11-сообщение
  `remove: ID="…"` (_NET_STARTUP_INFO_BEGIN/_NET_STARTUP_INFO по 20 байт), как GTK.
  `x11.go` собирается при `linux && cgo && (x11 || !wayland)`; с одним тегом `wayland` вместо него
  заглушка `x11_other.go` — сборка «только Wayland» не линкуется с libX11.
- `internal/service` — `systemctl` через exec, без root (polkit). `Start` сначала останавливает
  другие `zapret-dy@*` (общие таблица nft и очередь 200), после `start` ~1.5 с следит, что юнит
  не ушёл в failed/auto-restart (Type=simple отвечает «ок» до разбора аргументов nfqws).
  `Watch` (`watch.go`) — сигналы systemd по системной шине D-Bus (godbus, Subscribe + PropertiesChanged
  по точным путям юнитов из index.txt); тест — на пользовательском systemd через `systemd-run --user`.
  `Active(ctx, slugs...)` спрашивает systemd по D-Bus: со slugs — `ListUnitsByNames` (~5 мс), без них —
  `ListUnitsByPatterns` (~40 мс: PID 1 перебирает все юниты системы, как и `systemctl list-units`);
  без шины — запасной путь через `systemctl`.
- `internal/strategies` — `/opt/zapret-dy/strategies/index.txt` (`<slug>\t<имя .bat>`).
- `internal/config` — `~/.config/zapret-toggle/config.json`: `strategy`, `stop_on_close` (по умолчанию true),
  `theme` (`dark`/`light`, пусто — как в системе).
- `internal/ui` — окно со шапкой, карточкой и строкой состояния, размер свободный (по умолчанию 420x620, F11 — полный экран).
  `theme.go` — тема Fyne: палитры graphite/paper, шрифт Geist (`assets/fonts`),
  свои имена цветов `zt.*`. `widgets.go` — `box`/`text`/`icon`/`iconButton`: цвет берут из темы
  в `Refresh`, поэтому смена темы перекрашивает их сама. Иконки Lucide в `icons/` рисуются через
  stroke — Fyne перекрашивает только fill, так что цвет подставляет `iconRes`. `layouts.go` —
  раскладки без отступов Fyne (полосы 46/30/28 px). `PowerButton` (canvas.Circle/Arc
  + Tappable) растёт с окном от 136 до 260 px. Состояние окна
  трогается только в UI-потоке; systemctl — в горутинах, обратно через `fyne.Do`. Статус перечитывается
  по сигналам D-Bus (плюс страховочный опрос раз в 30 с), без D-Bus — опрос каждые 2 с. Обычный запрос
  статуса — только по стратегиям из списка, страховочный — по всем `zapret-dy@*`. `seq` отбрасывает
  результаты опроса, начатого до операции. `setStatus`/`SetState` без изменений не перерисовывают.
- `system/` — то, что ставится от root в `/opt/zapret-dy` и `/etc`: `firewall.sh` (nft, таблица
  `inet zapretdy`, один `nft -f`), `run.sh` (exec nfqws), `zapret-dy@.service`, `50-zapret-dy.rules.in`,
  `convert.py` (готов и проверен — не переписывать без необходимости).
- `scripts/` — `install.sh`, `uninstall.sh`, `update-flowseal.sh`, `check-strategies.sh`
  (белый список опций nfqws и путей только в `fake/`/`lists/`, затем `nfqws --dry-run`; не прошедшие
  выкидываются). Белый список — граница доверия к Flowseal: nfqws стартует от root, а getopt_long
  принимает сокращения, поэтому имена опций сверяются целиком.
  - `install.sh` название дистрибутива не проверяет — только возможности: x86_64, systemd
    (`/run/systemd/system`), `nft` (под root — `nft list tables`), polkit 0.106+ (версия —
    по `pkaction --version` через `polkit_supports_js_rules` из `platform.sh`, без неё — по каталогам
    `rules.d`), python3, make, runuser, getent и зависимости сборки. Все проблемы собираются в список и печатаются
    разом с одной командой установки пакетов; сам он пакеты не ставит. `--check` — только проверки,
    можно без sudo; замечания с «ВНИМАНИЕ» (например, нет модуля ядра) не ошибка, но итог тогда
    «пройдена с замечаниями». `restorecon` — только если SELinux включён (`selinuxenabled`/`getenforce`)
    и команда есть. Каталог polkit-правил, если его нет: с группой `polkitd` — `root:polkitd 750`,
    без неё — `root:root 755`.
  - `platform.sh` (подключается через source) — пакетный менеджер по `ID`/`ID_LIKE` из os-release
    (dnf, apt, pacman, zypper) и имена пакетов для подсказок. Таблица пакетов продублирована
    в README («Зависимости») — менять вместе.
  - `build-deps.sh` — `tags` (теги сборки по заголовкам), `check` (Go, компилятор C, pkg-config,
    OpenGL, заголовки под теги), `report` (то же для `install.sh`, который запускает его от имени
    пользователя), `cflags` (пути `-I` для CGO_CFLAGS).
  - `test-scripts.sh` — тесты `platform.sh` и `build-deps.sh` с подменными os-release, go, gcc
    и pkg-config; входит в `make test`.
- `third_party/` — nfqws и Flowseal; руками не правим, Flowseal обновляет `update-flowseal.sh`.

## Инварианты

- Юнит **без `[Install]`**, никакого автозапуска.
- Регулярка slug `^[a-z0-9-]+$` продублирована в `run.sh`, `firewall.sh`, polkit-правиле,
  `strategies.ValidSlug` и `check-strategies.sh` — менять везде сразу.
- fwmark `0x40000000` и очередь `200` — одинаковые в `run.sh`, `firewall.sh` и `check-strategies.sh`.
- Права в `/opt/zapret-dy`: каталоги 755, файлы 644, исполняемые 755 — nfqws работает не от root.
- Только YouTube и Discord: без ipset-all, GameFilter, Cloudflare-списков (это делает convert.py).
- sudo в этой среде не запускать; install/uninstall проверяются shellcheck, `scripts/install.sh --check`
  и прогоном в контейнере (см. «Проверено»).
- Никаких проверок вида `ID=fedora`: скрипты смотрят на возможности системы, а дистрибутив нужен
  только для подсказки, какой командой поставить пакеты. Отсутствие SELinux и `restorecon` — не ошибка.
- `go` в go.mod не понижать: Go от 1.21 сам скачивает нужный тулчейн (`export GOTOOLCHAIN ?= auto`
  в Makefile; при достаточно новом Go это ничего не меняет), для более старого — понятная ошибка.
- Теги сборки выбирает Makefile через `scripts/build-deps.sh tags`: есть заголовки X11 и Wayland —
  без тегов (оба бэкенда GLFW), только X11 — `x11`, только Wayland — `wayland`, ничего — ошибка
  с подсказкой. Модули pkg-config: X11 — `x11 xcursor xrandr xinerama xi xxf86vm`, Wayland —
  `wayland-client wayland-cursor wayland-egl xkbcommon egl`. Явные `TAGS=x11`/`TAGS=wayland`/`TAGS=`
  (оба) имеют приоритет.

## Проверено

- Все 22 стратегии проходят `nfqws --dry-run`. Под root dry-run тоже сбрасывает права (uid 0x7FFFFFFF),
  поэтому проверка в install.sh идёт после выставления прав.
- Hardening юнита не ломает nft/nfqws (проверено в `unshare --user --net` + `setpriv --no-new-privs
  --bounding-set`): nfqws сбрасывает uid, оставляет CAP_NET_ADMIN|CAP_NET_RAW, биндится к очереди 200.
  CAP_SETPCAP не нужен. Фильтры юнита подобраны по трассировке (ptrace) firewall.sh и nfqws под трафиком
  (STUN, TCP SYN, SIGHUP): все системные вызовы входят в `@system-service`, сокеты — AF_UNIX, AF_NETLINK,
  AF_INET, AF_INET6, на запись ничего не открывается. SELinux: `init_t` → `unconfined_service_t`
  с `nnp_transition` разрешён. `MemoryDenyWriteExecute` включать нельзя: nfqws упакован и делает память
  исполняемой через mprotect — падал с SIGSEGV сразу после старта. Настройки юнита можно прогнать без
  root: `systemd-run --user --wait --pipe -p <настройка>… /opt/zapret-dy/nfqws --dry-run …`.
- GLFW в Fyne 2.8 по умолчанию требует заголовки и X11, и Wayland; теги `x11`/`wayland` оставляют
  один бэкенд. GLFW подключает заголовки без pkg-config, поэтому: Wayland-бэкенду нужен ещё
  `EGL/egl.h` (в openSUSE — отдельный пакет `Mesa-libEGL-devel`), а в openSUSE заголовки Wayland
  лежат в `/usr/include/wayland` — Makefile добавляет `pkg-config --cflags-only-I` в `CGO_CFLAGS`
  (только если там не пусто; на Fedora, Debian и Arch переменная не трогается).
- Другие дистрибутивы проверяются в rootless podman (сборка — от пользователя в контейнере):
  Debian 13, Ubuntu 24.04, Arch, openSUSE Tumbleweed — пакеты из подсказок, `make check`, сборки
  «только X11», «только Wayland» (бинарник без `NEEDED libX11`) и с обоими бэкендами; Go 1.22/1.24
  из Debian/Ubuntu сам скачивает 1.26.10. Полная установка — в контейнере с systemd
  (`podman run --systemd=always --cap-add NET_ADMIN,NET_RAW … /sbin/init`, скрипт от root
  с `SUDO_USER=<пользователь>`): юнит стартует, таблица nft создаётся и удаляется. В rootless-контейнере
  `nfqws --dry-run` падает с `setgroups: Invalid argument` — uid 0x7FFFFFFF не отображён в user
  namespace; только для такого теста nfqws в копии репозитория заменяется обёрткой с `--uid=0:0`.
- В оболочках, кроме GNOME, само окно не запускалось (только сборка и установка): работа в KDE,
  XFCE, sway и т. д. и нативный Wayland-бэкенд проверены не были.

## Команды

```bash
make check                        # gofmt + shellcheck + vet + test (скрипты и Go) + build
make build TAGS=x11               # явно только X11; TAGS=wayland — только Wayland; TAGS= — оба
scripts/install.sh --check        # проверки установщика без изменений в системе (можно без sudo)
scripts/build-deps.sh check       # чего не хватает для сборки
systemd-analyze verify system/zapret-dy@.service   # ругается на пути, пока /opt/zapret-dy не установлен
```

## Коммиты

Заголовок: `<тип>: <краткое описание на русском>`, например `feat: новая иконка приложения`.
Типы (Conventional Commits, набор `@commitlint/config-conventional`):

- `feat` — новая возможность;
- `fix` — исправление ошибки;
- `docs` — только документация;
- `style` — форматирование, поведение кода не меняется;
- `refactor` — переработка кода без изменения поведения;
- `perf` — ускорение или экономия памяти;
- `test` — тесты;
- `build` — сборка и зависимости;
- `ci` — настройка CI;
- `chore` — прочее обслуживание: скрипты, конфиги;
- `revert` — откат прошлого коммита.
