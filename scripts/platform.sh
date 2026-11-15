#!/bin/bash
# Общие функции scripts/*.sh: пакетный менеджер дистрибутива и подсказка, какие пакеты поставить.
# Подключается через source и ничего не делает сам; пакеты никто не ставит — только печатается команда.
#
# Для тестов (scripts/test-scripts.sh) переопределяются:
#   ZT_OS_RELEASE  — путь к os-release (по умолчанию /etc/os-release);
#   ZT_PKG_MANAGER — готовый ответ pkg_manager.

# pkg_manager печатает dnf, apt, pacman или zypper; пустая строка — дистрибутив не опознан.
# Сначала ID и ID_LIKE из os-release (Manjaro и CachyOS — «как arch», Mint — «как ubuntu»),
# затем — какой менеджер пакетов вообще установлен.
pkg_manager() {
	if [[ -n ${ZT_PKG_MANAGER-} ]]; then
		printf '%s\n' "$ZT_PKG_MANAGER"
		return 0
	fi
	local file=${ZT_OS_RELEASE:-/etc/os-release} line id='' like='' word
	local -a words
	if [[ -r $file ]]; then
		# Файл не выполняем (source), а читаем: нужны только две строки.
		while IFS= read -r line || [[ -n $line ]]; do
			case $line in
			ID=*) id=${line#ID=} ;;
			ID_LIKE=*) like=${line#ID_LIKE=} ;;
			esac
		done < "$file"
	fi
	read -ra words <<< "${id//[\"\']/} ${like//[\"\']/}"
	for word in "${words[@]}"; do
		case $word in
		fedora | rhel | centos) echo dnf && return 0 ;;
		debian | ubuntu) echo apt && return 0 ;;
		arch) echo pacman && return 0 ;;
		suse | opensuse | opensuse-* | sles) echo zypper && return 0 ;;
		esac
	done
	for word in dnf:dnf apt-get:apt pacman:pacman zypper:zypper; do
		if command -v "${word%%:*}" >/dev/null 2>&1; then
			echo "${word#*:}"
			return 0
		fi
	done
	return 0
}

# pkg_names <менеджер> <зависимость>… — имена пакетов через пробел. Зависимости: go, gcc, make,
# pkg-config, python3, nft, polkit, systemd, util-linux, coreutils, getent, gl, x11, wayland.
# Незнакомые зависимости пропускаются.
pkg_names() {
	local pm=$1 dep pkg
	local -a names=() uniq=()
	local -A seen=()
	shift
	for dep in "$@"; do
		case $pm:$dep in
		dnf:go) names+=(golang) ;;
		apt:go) names+=(golang-go) ;;
		*:go) names+=(go) ;;
		apt:gcc) names+=(gcc libc6-dev) ;;
		*:gcc) names+=(gcc) ;;
		*:make) names+=(make) ;;
		apt:pkg-config) names+=(pkg-config) ;;
		pacman:pkg-config) names+=(pkgconf) ;;
		*:pkg-config) names+=(pkgconf-pkg-config) ;;
		pacman:python3) names+=(python) ;;
		*:python3) names+=(python3) ;;
		*:nft) names+=(nftables) ;;
		apt:polkit) names+=(polkitd) ;;
		*:polkit) names+=(polkit) ;;
		*:systemd) names+=(systemd) ;;
		*:util-linux) names+=(util-linux) ;;
		*:coreutils) names+=(coreutils) ;;
		apt:getent) names+=(libc-bin) ;;
		dnf:getent) names+=(glibc-common) ;;
		*:getent) names+=(glibc) ;;
		dnf:gl) names+=(mesa-libGL-devel) ;;
		apt:gl) names+=(libgl1-mesa-dev) ;;
		pacman:gl) names+=(libglvnd) ;;
		zypper:gl) names+=(Mesa-libGL-devel) ;;
		apt:x11) names+=(libx11-dev libxcursor-dev libxrandr-dev libxinerama-dev libxi-dev libxxf86vm-dev) ;;
		pacman:x11) names+=(libx11 libxcursor libxrandr libxinerama libxi libxxf86vm) ;;
		*:x11) names+=(libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel) ;;
		apt:wayland) names+=(libwayland-dev libxkbcommon-dev libegl-dev) ;;
		pacman:wayland) names+=(wayland libxkbcommon) ;;
		zypper:wayland) names+=(wayland-devel libxkbcommon-devel Mesa-libEGL-devel) ;;
		*:wayland) names+=(wayland-devel libxkbcommon-devel) ;;
		esac
	done
	for pkg in "${names[@]}"; do
		[[ -z ${seen[$pkg]-} ]] || continue
		seen[$pkg]=1
		uniq+=("$pkg")
	done
	echo "${uniq[*]}"
}

# polkit_supports_js_rules <версия> — понимает ли polkit этой версии правила из rules.d
# (на JavaScript): они появились в 0.106, а с 121 номер версии пишется без «0.».
# Код 0 — да, 1 — нет, 2 — версию разобрать не удалось.
polkit_supports_js_rules() {
	local major minor
	[[ ${1-} =~ ^([0-9]+)(\.([0-9]+))?([^0-9]|$) ]] || return 2
	major=$((10#${BASH_REMATCH[1]}))
	minor=$((10#${BASH_REMATCH[3]:-0}))
	((major > 0 || minor >= 106))
}

# install_cmd <менеджер> — начало команды установки пакетов.
install_cmd() {
	case $1 in
	dnf) echo "sudo dnf install" ;;
	apt) echo "sudo apt install" ;;
	pacman) echo "sudo pacman -S --needed" ;;
	zypper) echo "sudo zypper install" ;;
	esac
}

# install_hint <зависимость>… — команда установки пакетов для этого дистрибутива (по строке
# с отступом); если он не опознан — варианты для всех четырёх менеджеров пакетов.
install_hint() {
	local pm pkgs
	pm=$(pkg_manager)
	if [[ -n $pm ]]; then
		pkgs=$(pkg_names "$pm" "$@")
		[[ -z $pkgs ]] || echo "    $(install_cmd "$pm") $pkgs"
		return 0
	fi
	for pm in dnf apt pacman zypper; do
		pkgs=$(pkg_names "$pm" "$@")
		[[ -z $pkgs ]] || echo "    $(install_cmd "$pm") $pkgs"
	done
}
