# zapret-toggle — заметки для Claude Code

Десктоп-приложение (Go + Fyne v2.8, Fedora/GNOME): одна кнопка включает/выключает zapret1 (nfqws v72.13)
только для YouTube и Discord, со стратегиями Flowseal. Интерфейс, README и сообщения — на русском.

## Архитектура

- `cmd/zapret-toggle` — точка входа; app ID `zapret-toggle` = имя .desktop (Wayland app_id),
  X11 WM_CLASS = `Zapret Toggle` (= `StartupWMClass`).
- `internal/service` — `systemctl` через exec, без root (polkit). `Start` сначала останавливает
  другие `zapret-dy@*` (общие таблица nft и очередь 200), после `start` ~1.5 с следит, что юнит
  не ушёл в failed/auto-restart (Type=simple отвечает «ок» до разбора аргументов nfqws).
- `internal/strategies` — `/opt/zapret-dy/strategies/index.txt` (`<slug>\t<имя .bat>`).
- `internal/config` — `~/.config/zapret-toggle/config.json`: `strategy`, `stop_on_close` (по умолчанию true).
- `internal/ui` — окно 320x420, `PowerButton` (canvas.Circle/Arc + Tappable). Состояние окна
  трогается только в UI-потоке; systemctl — в горутинах, обратно через `fyne.Do`. `seq` отбрасывает
  результаты опроса (каждые 2 с), начатого до операции.
- `system/` — то, что ставится от root в `/opt/zapret-dy` и `/etc`: `firewall.sh` (nft, таблица
  `inet zapretdy`, один `nft -f`), `run.sh` (exec nfqws), `zapret-dy@.service`, `50-zapret-dy.rules.in`,
  `convert.py` (готов и проверен — не переписывать без необходимости).
- `scripts/` — `install.sh`, `uninstall.sh`, `update-flowseal.sh`, `check-strategies.sh`
  (`nfqws --dry-run` по index.txt, не прошедшие выкидываются).
- `third_party/` — nfqws и Flowseal; руками не правим, Flowseal обновляет `update-flowseal.sh`.

## Инварианты

- Юнит **без `[Install]`**, никакого автозапуска.
- Регулярка slug `^[a-z0-9-]+$` продублирована в `run.sh`, `firewall.sh`, polkit-правиле,
  `strategies.ValidSlug` и `check-strategies.sh` — менять везде сразу.
- fwmark `0x40000000` и очередь `200` — одинаковые в `run.sh`, `firewall.sh` и `check-strategies.sh`.
- Права в `/opt/zapret-dy`: каталоги 755, файлы 644, исполняемые 755 — nfqws работает не от root.
- Только YouTube и Discord: без ipset-all, GameFilter, Cloudflare-списков (это делает convert.py).
- sudo в этой среде не запускать; install/uninstall проверяются shellcheck и сухим прогоном.

## Проверено

- Все 22 стратегии проходят `nfqws --dry-run`. Под root dry-run тоже сбрасывает права (uid 0x7FFFFFFF),
  поэтому проверка в install.sh идёт после выставления прав.
- Hardening юнита не ломает nft/nfqws (проверено в `unshare --user --net` + `setpriv --bounding-set`):
  nfqws сбрасывает uid, оставляет CAP_NET_ADMIN|CAP_NET_RAW, биндится к очереди 200. CAP_SETPCAP не нужен.
- GLFW в Fyne 2.8 по умолчанию требует заголовки Wayland; без них Makefile собирает с `-tags x11`.

## Команды

```bash
make check                        # gofmt + shellcheck + vet + test + build
make build TAGS=x11               # явно только X11
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
