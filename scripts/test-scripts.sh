#!/bin/bash
# Тесты scripts/platform.sh и scripts/build-deps.sh — без root и без настоящих пакетов:
# дистрибутив подменяется файлом os-release, а go, gcc и pkg-config — заглушками в PATH.
# Запускается из `make test`.
set -euo pipefail

REPO=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source-path=SCRIPTDIR source=platform.sh
. "$REPO/scripts/platform.sh"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
fails=0

fail() {
	fails=$((fails + 1))
	echo "FAIL: $*" >&2
}

# equal <название> <получено> <ожидалось>
equal() {
	[[ $2 == "$3" ]] || fail "$1: получено '$2', ожидалось '$3'"
}

# contains <название> <текст> <подстрока>; lacks — наоборот.
contains() {
	[[ $2 == *"$3"* ]] || fail "$1: нет '$3' в выводе: $2"
}
lacks() {
	[[ $2 != *"$3"* ]] || fail "$1: лишнее '$3' в выводе: $2"
}

# --- platform.sh: пакетный менеджер по os-release -------------------------------

# os_release <строка>… — записать подменный os-release.
os_release() {
	printf '%s\n' "$@" > "$tmp/os-release"
}

# manager_of <ожидалось> <строка os-release>…
manager_of() {
	local want=$1
	shift
	os_release "$@"
	equal "pkg_manager ($*)" "$(ZT_OS_RELEASE=$tmp/os-release pkg_manager)" "$want"
}

manager_of dnf 'NAME="Fedora Linux"' 'ID=fedora'
manager_of dnf 'ID="almalinux"' 'ID_LIKE="rhel centos fedora"'
manager_of apt 'ID=debian'
manager_of apt 'ID=ubuntu' 'ID_LIKE=debian'
manager_of apt 'ID=linuxmint' 'ID_LIKE="ubuntu debian"'
manager_of pacman 'ID=arch'
manager_of pacman 'ID=manjaro' 'ID_LIKE=arch'
manager_of pacman 'ID=cachyos' 'ID_LIKE=arch'
manager_of zypper 'ID="opensuse-tumbleweed"' 'ID_LIKE="opensuse suse"'
manager_of zypper 'ID="opensuse-leap"' 'ID_LIKE="suse opensuse"'

# Незнакомый дистрибутив — по установленному менеджеру пакетов; нет никакого — пусто.
mkdir "$tmp/pm" "$tmp/empty"
printf '#!/bin/sh\n' > "$tmp/pm/pacman"
chmod 755 "$tmp/pm/pacman"
os_release 'ID=void'
equal "pkg_manager по команде" "$(ZT_OS_RELEASE=$tmp/os-release PATH=$tmp/pm pkg_manager)" pacman
equal "pkg_manager неизвестен" "$(ZT_OS_RELEASE=$tmp/os-release PATH=$tmp/empty pkg_manager)" ""
equal "pkg_manager без os-release" "$(ZT_OS_RELEASE=$tmp/нет PATH=$tmp/pm pkg_manager)" pacman

# --- platform.sh: пакеты и подсказка ------------------------------------------------

equal "pkg_names dnf" "$(pkg_names dnf go gcc gl wayland)" \
	"golang gcc mesa-libGL-devel wayland-devel libxkbcommon-devel"
equal "pkg_names apt" "$(pkg_names apt go gcc pkg-config python3 nft polkit)" \
	"golang-go gcc libc6-dev pkg-config python3 nftables polkitd"
equal "pkg_names pacman" "$(pkg_names pacman go python3 pkg-config gl)" "go python pkgconf libglvnd"
equal "pkg_names zypper" "$(pkg_names zypper go gl x11)" \
	"go Mesa-libGL-devel libX11-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel"
equal "pkg_names без повторов" "$(pkg_names apt go nft go неизвестное nft)" "golang-go nftables"
equal "install_hint apt" "$(ZT_PKG_MANAGER=apt install_hint nft wayland)" \
	"    sudo apt install nftables libwayland-dev libxkbcommon-dev libegl-dev"
equal "install_hint pacman" "$(ZT_PKG_MANAGER=pacman install_hint make)" "    sudo pacman -S --needed make"
equal "install_hint пустой" "$(ZT_PKG_MANAGER=dnf install_hint)" ""
hint=$(ZT_OS_RELEASE=$tmp/os-release PATH=$tmp/empty install_hint go)
for want in "sudo dnf install golang" "sudo apt install golang-go" "sudo pacman -S --needed go" "sudo zypper install go"; do
	contains "install_hint без менеджера" "$hint" "$want"
done

equal "pkg_names getent" \
	"$(pkg_names apt getent) $(pkg_names dnf getent) $(pkg_names pacman getent) $(pkg_names zypper getent)" \
	"libc-bin glibc-common glibc glibc"

# --- platform.sh: версия polkit --------------------------------------------------------

# polkit_code <версия> — код polkit_supports_js_rules: 0 — правила rules.d есть, 1 — нет, 2 — не версия.
polkit_code() {
	local code=0
	polkit_supports_js_rules "$1" || code=$?
	echo "$code"
}

equal "polkit 0.105" "$(polkit_code 0.105)" 1
equal "polkit 0.9" "$(polkit_code 0.9)" 1
for ver in 0.106 0.122 127 1.0 0.120.1 126-1; do
	equal "polkit $ver" "$(polkit_code "$ver")" 0
done
for ver in "" мусор version v127 .105; do
	equal "polkit '$ver' — не версия" "$(polkit_code "$ver")" 2
done

# --- build-deps.sh: заглушки go, gcc и pkg-config -------------------------------------

mkdir "$tmp/bin"
# pkg-config --exists <модуль>: модуль «установлен», если он есть в FAKE_PC;
# --cflags-only-I печатает FAKE_CFLAGS.
cat > "$tmp/bin/pkg-config" <<'EOF'
#!/bin/bash
case $1 in
--exists)
	shift
	for m in "$@"; do
		[[ " $FAKE_PC " == *" $m "* ]] || exit 1
	done
	;;
--cflags-only-I) echo "${FAKE_CFLAGS-}" ;;
*) exit 1 ;;
esac
EOF
cat > "$tmp/bin/go" <<'EOF'
#!/bin/bash
case $1 in
version) echo "go version go$FAKE_GO linux/amd64" ;;
env) echo gcc ;;
esac
EOF
printf '#!/bin/sh\n' > "$tmp/bin/gcc"
chmod 755 "$tmp/bin"/*

X11="x11 xcursor xrandr xinerama xi xxf86vm"
WAYLAND="wayland-client wayland-cursor wayland-egl xkbcommon egl"
NEW_GO=99.0.0 # новее любого go.mod

# deps <модули pkg-config> <версия go> <аргументы build-deps.sh>… — вывод (stdout и stderr) в out, код в rc.
# Перед вызовом можно задать окружение: GOTOOLCHAIN=local deps …
deps() {
	local pc=$1 gover=$2
	shift 2
	rc=0
	out=$(env -u GO -u CC -u PKG_CONFIG -u DISPLAY -u WAYLAND_DISPLAY -u XDG_SESSION_TYPE \
		PATH="$tmp/bin:/usr/bin:/bin" ZT_PKG_MANAGER=apt FAKE_PC="$pc" FAKE_GO="$gover" \
		"$REPO/scripts/build-deps.sh" "$@" 2>&1) || rc=$?
}
unset GOTOOLCHAIN

deps "gl $X11 $WAYLAND" "$NEW_GO" tags
equal "tags: X11 и Wayland" "$rc:$out" "0:"
deps "gl $X11" "$NEW_GO" tags
equal "tags: только X11" "$rc:$out" "0:x11"
deps "gl $WAYLAND" "$NEW_GO" tags
equal "tags: только Wayland" "$rc:$out" "0:wayland"
deps "gl ${X11/ xi / } $WAYLAND" "$NEW_GO" tags
equal "tags: неполный X11 не считается" "$rc:$out" "0:wayland"
deps "gl $X11 ${WAYLAND% egl}" "$NEW_GO" tags
equal "tags: Wayland без EGL не считается" "$rc:$out" "0:x11"
deps "gl" "$NEW_GO" tags
equal "tags: нет бэкендов — код" "$rc" 1
contains "tags: нет бэкендов" "$out" "ни X11"
contains "tags: нет бэкендов — пакеты" "$out" "sudo apt install libx11-dev"
contains "tags: нет бэкендов — Wayland" "$out" "libwayland-dev"

deps "gl $X11 $WAYLAND" "$NEW_GO" check
equal "check: всё есть — код" "$rc" 0
contains "check: всё есть" "$out" "Go $NEW_GO, gcc, окно — X11 и Wayland"
deps "gl $X11" "$NEW_GO" check
contains "check: только X11" "$rc:$out" "0:сборка: Go $NEW_GO, gcc, окно — только X11"
contains "check: только X11 — про XWayland" "$out" "XWayland"
deps "gl $WAYLAND" "$NEW_GO" check
contains "check: только Wayland" "$rc:$out" "0:сборка: Go $NEW_GO, gcc, окно — только Wayland"

# Теги заданы явно: проверяются заголовки именно под них.
deps "gl $X11" "$NEW_GO" check wayland
equal "check wayland без заголовков — код" "$rc" 1
contains "check wayland без заголовков" "$out" "sudo apt install libwayland-dev libxkbcommon-dev libegl-dev"
deps "gl $X11" "$NEW_GO" check ''
equal "check '' (оба бэкенда) без Wayland — код" "$rc" 1
deps "gl $X11 $WAYLAND" "$NEW_GO" check x11
contains "check x11 при обоих наборах" "$rc:$out" "окно — только X11"
deps "gl $X11" "$NEW_GO" check none
equal "check none — код" "$rc" 1

deps "$X11" "$NEW_GO" check
equal "check: нет OpenGL — код" "$rc" 1
contains "check: нет OpenGL" "$out" "sudo apt install libgl1-mesa-dev"

# Go старее go.mod: с 1.21 нужную версию скачает сам, если это не запрещено; до 1.21 — ошибка.
deps "gl $X11" 1.21.0 check
equal "check: Go 1.21 — код" "$rc" 0
contains "check: Go 1.21" "$out" "GOTOOLCHAIN=auto"
GOTOOLCHAIN=local deps "gl $X11" 1.21.0 check
equal "check: Go 1.21 и GOTOOLCHAIN=local — код" "$rc" 1
contains "check: Go 1.21 и GOTOOLCHAIN=local" "$out" "GOTOOLCHAIN=local"
deps "gl $X11" 1.20.14 check
equal "check: Go 1.20 — код" "$rc" 1
contains "check: Go 1.20" "$out" "слишком старый"
lacks "check: Go 1.20 — пакет не поможет" "$out" "golang-go"

# Нет Go, компилятора и pkg-config — одна команда на всё.
rc=0
out=$(env -u GOTOOLCHAIN PATH="$tmp/empty:/usr/bin:/bin" ZT_PKG_MANAGER=apt GO=нет-go CC=нет-cc PKG_CONFIG=нет-pkg-config \
	"$REPO/scripts/build-deps.sh" check 2>&1) || rc=$?
equal "check: ничего нет — код" "$rc" 1
contains "check: ничего нет" "$out" \
	"sudo apt install golang-go gcc libc6-dev pkg-config libgl1-mesa-dev libx11-dev"

# cflags: пути к заголовкам не из /usr/include (openSUSE) — только для бэкендов, которые собираются.
FAKE_CFLAGS=-I/usr/include/wayland deps "gl $X11 $WAYLAND" "$NEW_GO" cflags
equal "cflags: оба бэкенда" "$rc:$out" "0:-I/usr/include/wayland"
FAKE_CFLAGS=-I/usr/include/wayland deps "gl $X11" "$NEW_GO" cflags wayland
equal "cflags: бэкенда нет" "$rc:$out" "0:"
deps "gl $X11 $WAYLAND" "$NEW_GO" cflags
equal "cflags: всё в /usr/include" "$rc:$out" "0:"
deps "gl" "$NEW_GO" cflags none
equal "cflags: нет заголовков" "$rc:$out" "0:"

# report — для install.sh: строки «вид<TAB>текст», в конце done, код 0 даже при нехватке.
deps "gl" "$NEW_GO" report
equal "report — код" "$rc" 0
equal "report — последняя строка" "${out##*$'\n'}" "done"
contains "report — problem" "$out" $'problem\tнет заголовков ни X11'
contains "report — need" "$out" $'need\tx11'

if ((fails)); then
	echo "test-scripts.sh: не прошло проверок: $fails" >&2
	exit 1
fi
echo "ok  	scripts/platform.sh, scripts/build-deps.sh"
