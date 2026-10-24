#!/bin/bash
# Установка zapret-toggle (идемпотентно, можно запускать повторно):
#   * системная часть в /opt/zapret-dy: nfqws, firewall.sh, run.sh, фейки, списки и стратегии
#     (convert.py + проверка nfqws --dry-run), systemd-юнит zapret-dy@.service, polkit-правило;
#   * приложение — от имени пользователя: ~/.local/bin, ~/.local/share/applications, иконка.
#
#   sudo scripts/install.sh                    — всё
#   sudo scripts/install.sh --strategies-only  — только обновить фейки/списки/стратегии
#
# Автозапуск не настраивается: у юнита нет [Install], zapret включается только кнопкой.
set -euo pipefail

REPO=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
BASE=/opt/zapret-dy
UNIT=/etc/systemd/system/zapret-dy@.service
RULE=/etc/polkit-1/rules.d/50-zapret-dy.rules
FLOWSEAL=$REPO/third_party/flowseal
NFQWS=$REPO/third_party/zapret/nfqws

die() {
	echo "install.sh: ошибка: $*" >&2
	exit 1
}
step() { echo "==> $*"; }

strategies_only=0
case ${1-} in
"") ;;
--strategies-only) strategies_only=1 ;;
*) die "неизвестный аргумент: $1 (есть только --strategies-only)" ;;
esac

# --- проверки ---------------------------------------------------------------

[[ $EUID -eq 0 ]] || die "нужен root: sudo $0"
user=${SUDO_USER-}
[[ -n $user && $user != root ]] || die "запусти через sudo из-под своего пользователя (не из root-шелла): нужен SUDO_USER"
# Имя попадает в polkit-правило (JS-строка) — пускаем только обычные имена.
[[ $user =~ ^[a-z_][a-z0-9_.-]*$ ]] || die "неподдерживаемое имя пользователя: '$user'"
home=$(getent passwd "$user" | cut -d: -f6)
[[ -n $home && -d $home ]] || die "не найден домашний каталог пользователя $user"

# shellcheck source=/dev/null
. /etc/os-release
[[ ${ID-} == fedora ]] || die "поддерживается только Fedora (ID=${ID-?})"

for cmd in nft python3 systemctl restorecon runuser install make; do
	command -v "$cmd" >/dev/null || die "не найдена команда '$cmd'"
done
[[ -x $NFQWS ]] || die "нет $NFQWS"
[[ -f $FLOWSEAL/COMMIT ]] || die "нет $FLOWSEAL (скачай: scripts/update-flowseal.sh)"
compgen -G "$FLOWSEAL/fake/*.bin" >/dev/null || die "нет фейковых пакетов в $FLOWSEAL/fake"

as_user() { runuser -u "$user" -- "$@"; }

if ((!strategies_only)); then
	runuser -l "$user" -c 'command -v go' >/dev/null ||
		die "у пользователя $user не найден Go (нужен для сборки приложения): sudo dnf install golang"
fi

umask 022

# --- системная часть -----------------------------------------------------------

install_strategies() {
	step "Фейковые пакеты → $BASE/fake"
	install -d -m 755 "$BASE" "$BASE/fake"
	rm -f "$BASE"/fake/*.bin
	install -m 644 "$FLOWSEAL"/fake/*.bin "$BASE/fake/"

	step "Списки и стратегии (convert.py, Flowseal $(head -c 12 "$FLOWSEAL/COMMIT"))"
	python3 -I "$REPO/system/convert.py" "$FLOWSEAL" "$BASE"

	fix_permissions

	# Под root nfqws --dry-run сбрасывает права так же, как при работе, — заодно проверяется,
	# что списки и фейки читаются непривилегированным пользователем.
	step "Проверка стратегий: nfqws --dry-run"
	"$REPO/scripts/check-strategies.sh" "$BASE" "$BASE/nfqws"
}

# nfqws сбрасывает права до непривилегированного пользователя — всё должно читаться всеми.
fix_permissions() {
	chown -R root:root "$BASE"
	find "$BASE" -type d -exec chmod 755 {} +
	find "$BASE" -type f -exec chmod 644 {} +
	chmod 755 "$BASE/nfqws" "$BASE/firewall.sh" "$BASE/run.sh"
}

if ((!strategies_only)); then
	step "nfqws и скрипты → $BASE"
	install -d -m 755 "$BASE"
	install -m 755 "$NFQWS" "$BASE/nfqws"
	install -m 755 "$REPO/system/firewall.sh" "$BASE/firewall.sh"
	install -m 755 "$REPO/system/run.sh" "$BASE/run.sh"
else
	[[ -x $BASE/nfqws ]] || die "$BASE не установлен — запусти без --strategies-only"
fi

install_strategies

if ((!strategies_only)); then
	step "systemd-юнит → $UNIT"
	install -m 644 "$REPO/system/zapret-dy@.service" "$UNIT"

	step "polkit-правило для пользователя $user → $RULE"
	# На Fedora каталог есть (root:polkitd 750); создаём так же, если его нет.
	[[ -d $(dirname "$RULE") ]] || install -d -m 750 -g polkitd "$(dirname "$RULE")"
	sed "s/@USER@/$user/g" "$REPO/system/50-zapret-dy.rules.in" > "$RULE.tmp"
	chmod 644 "$RULE.tmp"
	mv -f "$RULE.tmp" "$RULE"

	systemctl daemon-reload
fi

step "SELinux: restorecon"
restorecon -R "$BASE"
((strategies_only)) || restorecon "$UNIT" "$RULE"

# Если zapret сейчас включён — перезапустить на новых файлах (или выключить, если стратегии больше нет).
while read -r unit _; do
	[[ $unit == zapret-dy@*.service ]] || continue
	slug=${unit#zapret-dy@}
	slug=${slug%.service}
	if [[ -f $BASE/strategies/$slug.args ]]; then
		step "Перезапуск $unit"
		systemctl restart "$unit" || echo "ПРЕДУПРЕЖДЕНИЕ: не удалось перезапустить $unit" >&2
	else
		step "Стратегии $slug больше нет — остановка $unit"
		systemctl stop "$unit" || true
	fi
done < <(systemctl list-units 'zapret-dy@*' --state=active --plain --no-legend)

if ((strategies_only)); then
	step "Готово: стратегии обновлены"
	exit 0
fi

# --- приложение (от имени пользователя) --------------------------------------------

step "Сборка приложения (make build от $user)"
runuser -l "$user" -c "make -C $(printf '%q' "$REPO") build"

bindir=$home/.local/bin
appdir=$home/.local/share/applications
icondir=$home/.local/share/icons/hicolor/256x256/apps

step "Приложение → $bindir/zapret-toggle"
as_user install -D -m 755 "$REPO/build/zapret-toggle" "$bindir/zapret-toggle"
as_user install -D -m 644 "$REPO/assets/icon.png" "$icondir/zapret-toggle.png"
# ~/.local/bin может не быть в PATH у GNOME Shell — в ярлыке абсолютный путь.
as_user install -d -m 755 "$appdir"
sed "s|^Exec=.*|Exec=$bindir/zapret-toggle|" "$REPO/assets/zapret-toggle.desktop" |
	as_user tee "$appdir/zapret-toggle.desktop.tmp" >/dev/null
as_user chmod 644 "$appdir/zapret-toggle.desktop.tmp"
as_user mv -f "$appdir/zapret-toggle.desktop.tmp" "$appdir/zapret-toggle.desktop"
if command -v update-desktop-database >/dev/null; then
	as_user update-desktop-database "$appdir" || true
fi

cat <<EOF

Готово. Запусти «Zapret Toggle» из меню приложений (или $bindir/zapret-toggle)
и нажми на кнопку. В автозагрузку ничего не добавлено.
EOF
