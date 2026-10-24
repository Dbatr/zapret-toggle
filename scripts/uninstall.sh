#!/bin/bash
# Удаление zapret-toggle: останавливает zapret-dy@*, удаляет /opt/zapret-dy, юнит, polkit-правило
# и приложение пользователя.
#
#   sudo scripts/uninstall.sh          — всё, кроме настроек приложения
#   sudo scripts/uninstall.sh --purge  — и настройки (~/.config/zapret-toggle)
set -euo pipefail

BASE=/opt/zapret-dy
UNIT=/etc/systemd/system/zapret-dy@.service
RULE=/etc/polkit-1/rules.d/50-zapret-dy.rules

die() {
	echo "uninstall.sh: ошибка: $*" >&2
	exit 1
}
step() { echo "==> $*"; }

purge=0
case ${1-} in
"") ;;
--purge) purge=1 ;;
*) die "неизвестный аргумент: $1 (есть только --purge)" ;;
esac

[[ $EUID -eq 0 ]] || die "нужен root: sudo $0"

while read -r unit _; do
	[[ $unit == zapret-dy@*.service ]] || continue
	step "Остановка $unit"
	systemctl stop "$unit" || true
done < <(systemctl list-units 'zapret-dy@*' --all --plain --no-legend)

# На случай, если таблица осталась (ExecStopPost не отработал).
if nft list table inet zapretdy >/dev/null 2>&1; then
	step "Удаление таблицы nftables inet zapretdy"
	nft delete table inet zapretdy
fi

step "Удаление юнита и polkit-правила"
rm -f "$UNIT" "$RULE"
systemctl daemon-reload
systemctl reset-failed 'zapret-dy@*' 2>/dev/null || true

step "Удаление $BASE"
rm -rf --one-file-system "$BASE"

user=${SUDO_USER-}
if [[ -n $user && $user != root ]]; then
	home=$(getent passwd "$user" | cut -d: -f6)
	if [[ -n $home && -d $home ]]; then
		step "Удаление приложения пользователя $user"
		rm -f "$home/.local/bin/zapret-toggle" \
			"$home/.local/share/applications/zapret-toggle.desktop" \
			"$home/.local/share/icons/hicolor/256x256/apps/zapret-toggle.png"
		if command -v update-desktop-database >/dev/null; then
			runuser -u "$user" -- update-desktop-database "$home/.local/share/applications" || true
		fi
		if ((purge)); then
			step "Удаление настроек"
			rm -rf "$home/.config/zapret-toggle" "$home/.config/fyne/zapret-toggle"
		fi
	fi
else
	echo "SUDO_USER не задан — файлы приложения в ~/.local не тронуты" >&2
fi

step "Готово"
