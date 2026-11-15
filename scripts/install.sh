#!/bin/bash
# Установка zapret-toggle (идемпотентно, можно запускать повторно):
#   * системная часть в /opt/zapret-dy: nfqws, firewall.sh, run.sh, фейки, списки и стратегии
#     (convert.py + проверка nfqws --dry-run), systemd-юнит zapret-dy@.service, polkit-правило;
#   * приложение — от имени пользователя: ~/.local/bin, ~/.local/share/applications, иконка.
#
#   sudo scripts/install.sh                    — всё
#   sudo scripts/install.sh --strategies-only  — только обновить фейки/списки/стратегии
#   scripts/install.sh --check                 — только проверить систему и зависимости, ничего
#                                                не меняя (можно без sudo; под sudo проверяется
#                                                ещё и работа nftables)
#
# Дистрибутив не важен — проверяются возможности: x86_64, systemd, nftables, polkit с правилами
# rules.d, python3, make и то, что нужно для сборки приложения (scripts/build-deps.sh).
# Недостающие пакеты скрипт сам не ставит, а печатает команду для dnf, apt, pacman или zypper.
#
# Автозапуск не настраивается: у юнита нет [Install], zapret включается только кнопкой.
set -euo pipefail

REPO=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
BASE=/opt/zapret-dy
UNIT=/etc/systemd/system/zapret-dy@.service
RULE=/etc/polkit-1/rules.d/50-zapret-dy.rules
FLOWSEAL=$REPO/third_party/flowseal
NFQWS=$REPO/third_party/zapret/nfqws

# В Debian и openSUSE у обычного пользователя в PATH нет sbin (nft, runuser) — это для --check без sudo.
PATH=$PATH:/usr/local/sbin:/usr/sbin:/sbin

# shellcheck source-path=SCRIPTDIR source=platform.sh
. "$REPO/scripts/platform.sh"

die() {
	echo "install.sh: ошибка: $*" >&2
	exit 1
}
step() { echo "==> $*"; }
warn() { echo "ПРЕДУПРЕЖДЕНИЕ: $*" >&2; }

strategies_only=0
check_only=0
case ${1-} in
"") ;;
--strategies-only) strategies_only=1 ;;
--check) check_only=1 ;;
*) die "неизвестный аргумент: $1 (есть --strategies-only и --check)" ;;
esac

# --- проверки ---------------------------------------------------------------

if [[ $EUID -eq 0 ]]; then
	user=${SUDO_USER-}
	[[ -n $user && $user != root ]] || die "запусти через sudo из-под своего пользователя (не из root-шелла): нужен SUDO_USER"
elif ((check_only)); then
	user=$(id -un)
else
	die "нужен root: sudo $0 (проверить систему, ничего не меняя: $0 --check)"
fi
# Имя попадает в polkit-правило (JS-строка) — пускаем только обычные имена.
[[ $user =~ ^[a-z_][a-z0-9_.-]*$ ]] || die "неподдерживаемое имя пользователя: '$user'"
home=$(getent passwd "$user" | cut -d: -f6)
[[ -n $home && -d $home ]] || die "не найден домашний каталог пользователя $user"

as_user() { runuser -u "$user" -- "$@"; }

# Команда оболочки от имени пользователя в его окружении входа (там PATH с его Go).
# Без root (--check) пользователь и так он сам.
user_sh() {
	if [[ $EUID -eq 0 ]]; then
		runuser -l "$user" -c "$1"
	else
		bash -c "$1"
	fi
}

problems=() # что мешает установке
missing=()  # чего не хватает — имена зависимостей для install_hint (platform.sh)
notes=()    # что стоит знать, но установке не мешает
problem() { problems+=("$1"); }

have() { command -v "$1" >/dev/null 2>&1; }

# need_cmd <команда> <зависимость> — нет команды: записать проблему и пакет для подсказки.
need_cmd() {
	if ! have "$1"; then
		problem "не найдена команда '$1'"
		missing+=("$2")
	fi
}

# SELinux включён (в любом режиме, кроме disabled)? Тогда после установки нужен restorecon.
selinux_enabled() {
	if have selinuxenabled; then
		selinuxenabled
	elif have getenforce; then
		[[ $(getenforce 2>/dev/null) != Disabled ]]
	else
		[[ -e /sys/fs/selinux/enforce ]]
	fi
}

# kmod_missing <модуль> — модуля ядра точно нет: не загружен, не встроен и не лежит в /lib/modules.
# Если списка модулей нет (контейнер, своё ядро) — считаем, что судить не по чему.
kmod_missing() {
	local dir
	[[ ! -d /sys/module/$1 ]] || return 1
	dir=/lib/modules/$(uname -r)
	[[ -f $dir/modules.dep ]] || return 1
	! grep -qs "/$1\.ko" "$dir/modules.dep" "$dir/modules.builtin"
}

# Система: то, без чего не заработает /opt/zapret-dy и юнит.
check_system() {
	local arch mod
	arch=$(uname -m)
	# third_party/zapret/nfqws — статический бинарник под x86_64; на ARM нужен другой бинарник zapret.
	[[ $arch == x86_64 ]] || problem "архитектура $arch не поддерживается: nfqws в third_party собран только для x86_64"

	[[ -d /run/systemd/system ]] || problem "система загружена не через systemd (нет /run/systemd/system) — запускать юнит zapret-dy@.service некому"
	need_cmd systemctl systemd

	if have nft; then
		if [[ $EUID -ne 0 ]]; then
			notes+=("работа nftables без root не проверялась — полная проверка: sudo $0 --check")
		elif ! nft list tables >/dev/null 2>&1; then
			problem "nftables не работает: «nft list tables» завершилась ошибкой (в ядре нет nf_tables?)"
		fi
	else
		problem "не найдена команда 'nft' (nftables)"
		missing+=(nft)
	fi
	for mod in nf_tables nft_queue nfnetlink_queue; do
		if kmod_missing "$mod"; then
			notes+=("ВНИМАНИЕ: не найден модуль ядра $mod — без него правила zapret не загрузятся")
		fi
	done

	need_cmd python3 python3
	need_cmd runuser util-linux
	need_cmd install coreutils

	[[ -x $NFQWS ]] || problem "нет $NFQWS"
	if [[ ! -f $FLOWSEAL/COMMIT ]]; then
		problem "нет $FLOWSEAL (скачай: scripts/update-flowseal.sh)"
	else
		compgen -G "$FLOWSEAL/fake/*.bin" >/dev/null || problem "нет фейковых пакетов в $FLOWSEAL/fake"
		# Файлы Flowseal читает root и кладёт в /opt читаемыми всем: ссылка вытащила бы туда чужой файл.
		[[ -z $(find "$FLOWSEAL" -type l -print -quit) ]] || problem "в $FLOWSEAL есть символические ссылки"
	fi
}

# polkit: правило кладётся в /etc/polkit-1/rules.d, а такие правила (на JavaScript) понимает
# polkit 0.106 и новее — у него есть хотя бы /usr/share/polkit-1/rules.d.
check_polkit() {
	if [[ -d ${RULE%/*} || -d /usr/share/polkit-1/rules.d ]]; then
		return 0
	fi
	if have pkaction || [[ -d /etc/polkit-1 ]]; then
		problem "polkit слишком старый: нет каталога rules.d (нужен polkit 0.106+ с правилами на JavaScript; в Ubuntu 22.04 и Debian 11 его нет)"
	else
		problem "не найден polkit (нет ${RULE%/*})"
		missing+=(polkit)
	fi
}

# Сборка приложения: проверяет scripts/build-deps.sh от имени пользователя (его Go и pkg-config).
check_app() {
	local kind text finished=0
	need_cmd make make
	while IFS=$'\t' read -r kind text; do
		case $kind in
		problem) problem "$text" ;;
		need)
			missing+=("$text")
			# Go не из пакета должен быть в PATH оболочки входа, а не только в ~/.bashrc.
			[[ $text != go ]] || notes+=("если Go уже стоит (не из пакета), его каталог должен быть в PATH оболочки входа пользователя $user: ~/.profile или ~/.bash_profile")
			;;
		note) notes+=("$text") ;;
		summary) [[ -z $text ]] || notes+=("сборка приложения: $text") ;;
		done) finished=1 ;;
		esac
	done < <(user_sh "$(printf '%q' "$REPO/scripts/build-deps.sh") report" 2>/dev/null || true)
	((finished)) || problem "не удалось проверить зависимости сборки от имени $user (scripts/build-deps.sh check)"
}

step "Проверка системы и зависимостей"
check_system
if ((!strategies_only)); then
	check_polkit
	check_app
fi

if ((${#problems[@]})); then
	echo "install.sh: установка невозможна:" >&2
	printf '  - %s\n' "${problems[@]}" >&2
	if ((${#missing[@]})); then
		hint=$(install_hint "${missing[@]}")
		if [[ -n $hint ]]; then
			echo "Установи недостающее (скрипт пакеты не ставит):" >&2
			echo "$hint" >&2
		fi
	fi
	if ((${#notes[@]})); then
		echo "К сведению:" >&2
		printf '    %s\n' "${notes[@]}" >&2
	fi
	exit 1
fi

if ((check_only)); then
	if selinux_enabled; then
		if have restorecon; then
			notes+=("SELinux включён — метки файлов выставит restorecon")
		else
			notes+=("ВНИМАНИЕ: SELinux включён, но нет restorecon (пакет policycoreutils) — метки файлов не обновятся")
		fi
	else
		notes+=("SELinux не включён — шаг restorecon будет пропущен")
	fi
fi
if ((${#notes[@]})); then
	printf '    %s\n' "${notes[@]}"
fi
if ((check_only)); then
	echo "Проверка пройдена, можно ставить: sudo $0"
	exit 0
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
	# Каталог обычно создаёт пакет polkit. Если его нет — как в пакете: с группой polkitd
	# (Fedora, Arch, openSUSE) root:polkitd 750, без неё root:root 755, иначе polkitd правило не прочтёт.
	if [[ ! -d ${RULE%/*} ]]; then
		if getent group polkitd >/dev/null; then
			install -d -m 750 -g polkitd "${RULE%/*}"
		else
			install -d -m 755 "${RULE%/*}"
		fi
		# За каталогом, которого не было при старте, polkitd не следит — пусть перечитает правила.
		systemctl try-restart polkit.service 2>/dev/null || true
	fi
	sed "s/@USER@/$user/g" "$REPO/system/50-zapret-dy.rules.in" > "$RULE.tmp"
	chmod 644 "$RULE.tmp"
	mv -f "$RULE.tmp" "$RULE"

	systemctl daemon-reload
fi

# Метки SELinux нужны только там, где он включён (Fedora, RHEL, новые openSUSE); в Debian, Ubuntu
# и Arch его обычно нет, как и самого restorecon.
if ! selinux_enabled; then
	step "SELinux не включён — restorecon пропущен"
elif have restorecon; then
	step "SELinux: restorecon"
	restorecon -R "$BASE"
	((strategies_only)) || restorecon "$UNIT" "$RULE"
else
	warn "SELinux включён, но restorecon не найден (пакет policycoreutils) — метки файлов не обновлены"
fi

# Если zapret сейчас включён — перезапустить на новых файлах (или выключить, если стратегии больше нет).
while read -r unit _; do
	[[ $unit == zapret-dy@*.service ]] || continue
	slug=${unit#zapret-dy@}
	slug=${slug%.service}
	if [[ -f $BASE/strategies/$slug.args ]]; then
		step "Перезапуск $unit"
		systemctl restart "$unit" || warn "не удалось перезапустить $unit"
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
hicolor=$home/.local/share/icons/hicolor
icondir=$hicolor/256x256/apps

step "Приложение → $bindir/zapret-toggle"
as_user install -D -m 755 "$REPO/build/zapret-toggle" "$bindir/zapret-toggle"
as_user install -D -m 644 "$REPO/assets/icon.png" "$icondir/zapret-toggle.png"
# ~/.local/bin может не быть в PATH у оболочки рабочего стола — в ярлыке абсолютный путь.
as_user install -d -m 755 "$appdir"
sed "s|^Exec=.*|Exec=$bindir/zapret-toggle|" "$REPO/assets/zapret-toggle.desktop" |
	as_user tee "$appdir/zapret-toggle.desktop.tmp" >/dev/null
as_user chmod 644 "$appdir/zapret-toggle.desktop.tmp"
as_user mv -f "$appdir/zapret-toggle.desktop.tmp" "$appdir/zapret-toggle.desktop"
if have update-desktop-database; then
	as_user update-desktop-database "$appdir" || true
fi
# Оболочки (GTK, KDE) замечают новую иконку по времени изменения каталога темы. Кэш GTK обновляем,
# только если он уже есть (так же делает xdg-icon-resource): созданный здесь, он бы со временем
# устарел и прятал иконки программ, которые кэш не обновляют.
as_user touch "$hicolor" || true
if have gtk-update-icon-cache && as_user test -f "$hicolor/icon-theme.cache"; then
	as_user gtk-update-icon-cache -q -t -f "$hicolor" || true
fi

cat <<EOF

Готово. Запусти «Zapret Toggle» из меню приложений (или $bindir/zapret-toggle)
и нажми на кнопку. В автозагрузку ничего не добавлено.
EOF
