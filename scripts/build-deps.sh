#!/bin/bash
# Что нужно для сборки приложения (Go + Fyne: GLFW и OpenGL через cgo) и с какими тегами собирать.
#
#   build-deps.sh tags          — теги сборки по установленным заголовкам (их ищет pkg-config):
#                                   есть X11 и Wayland — пустая строка (GLFW соберёт оба бэкенда),
#                                   только X11 — x11, только Wayland — wayland (без X11-библиотек),
#                                   нет ни тех, ни других — ошибка (код 1) с подсказкой по пакетам;
#   build-deps.sh check [теги]  — проверить Go (версия из go.mod), компилятор C, pkg-config, OpenGL
#                                   и заголовки под эти теги (без аргумента — под те, что даёт tags):
#                                   всё есть — строка с итогом, нет — список и команда установки (код 1);
#   build-deps.sh report [теги] — то же для install.sh: строки «вид<TAB>текст» (problem, need, note,
#                                   summary) и в конце «done»; код всегда 0;
#   build-deps.sh cflags [теги] — флаги -I к заголовкам под эти теги, если они лежат не в /usr/include
#                                   (openSUSE: /usr/include/wayland): GLFW подключает заголовки без
#                                   pkg-config, поэтому Makefile добавляет эти флаги в CGO_CFLAGS.
#
# Окружение: GO (по умолчанию go), CC (как в go env, обычно gcc), PKG_CONFIG (pkg-config), GOTOOLCHAIN.
set -euo pipefail

REPO=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source-path=SCRIPTDIR source=platform.sh
. "$REPO/scripts/platform.sh"

# Модули pkg-config, без которых GLFW не соберёт бэкенд. egl — это EGL/egl.h: его подключает
# Wayland-часть GLFW, а в openSUSE он в отдельном пакете.
X11_MODULES=(x11 xcursor xrandr xinerama xi xxf86vm)
WAYLAND_MODULES=(wayland-client wayland-cursor wayland-egl xkbcommon egl)
# С этой версии go сам скачивает тулчейн, который требует go.mod (GOTOOLCHAIN=auto).
GO_TOOLCHAIN_MIN=1.21.0

GO=${GO:-go}
PKG_CONFIG=${PKG_CONFIG:-pkg-config}

problems=()
needs=()
notes=()
summary=''
problem() { problems+=("$1"); }
need() { needs+=("$@"); }
note() { notes+=("$1"); }

# missing_modules <модуль>… — каких модулей pkg-config не видит (через пробел).
missing_modules() {
	local m
	local -a miss=()
	for m in "$@"; do
		"$PKG_CONFIG" --exists "$m" 2>/dev/null || miss+=("$m")
	done
	echo "${miss[*]-}"
}

# auto_tags печатает теги по найденным заголовкам; код 1 — нет ни X11, ни Wayland.
auto_tags() {
	if [[ -z $x11_missing && -z $wayland_missing ]]; then
		echo
	elif [[ -z $x11_missing ]]; then
		echo x11
	elif [[ -z $wayland_missing ]]; then
		echo wayland
	else
		return 1
	fi
}

# ver_lt <a> <b> — версия a (x.y.z) меньше b.
ver_lt() {
	local -a a b
	local i
	IFS=. read -ra a <<< "$1"
	IFS=. read -ra b <<< "$2"
	for i in 0 1 2; do
		if ((${a[i]:-0} < ${b[i]:-0})); then return 0; fi
		if ((${a[i]:-0} > ${b[i]:-0})); then return 1; fi
	done
	return 1
}

# Go: есть ли он и соберёт ли go.mod (сам или скачав нужный тулчейн).
check_go() {
	local out have want='' key val
	if ! command -v "$GO" >/dev/null 2>&1; then
		problem "не найден Go (команда «$GO»)"
		need go
		return 0
	fi
	while read -r key val _; do
		if [[ $key == go ]]; then
			want=$val
			break
		fi
	done < "$REPO/go.mod"
	# Из корня и с GOTOOLCHAIN=local: нужна версия установленного Go, а не той, на которую он переключится.
	out=$(cd / && GOTOOLCHAIN=local "$GO" version 2>/dev/null) || out=''
	if [[ ! $out =~ go([0-9]+)\.([0-9]+)(\.([0-9]+))? ]]; then
		problem "не удалось узнать версию Go («$GO version»)"
		return 0
	fi
	have=${BASH_REMATCH[1]}.${BASH_REMATCH[2]}.${BASH_REMATCH[4]:-0}
	summary="Go $have"
	if ! ver_lt "$have" "$want"; then
		return 0
	fi
	if ver_lt "$have" "$GO_TOOLCHAIN_MIN"; then
		problem "Go $have слишком старый: нужен Go $want или новее (подойдёт и любой Go от ${GO_TOOLCHAIN_MIN%.0} — нужную версию он скачает сам). Свежий Go: https://go.dev/dl/"
	elif [[ ${GOTOOLCHAIN:-auto} == local ]]; then
		problem "Go $have старее, чем требует go.mod ($want), а GOTOOLCHAIN=local запрещает скачать нужный: убери GOTOOLCHAIN=local или поставь Go $want+ (https://go.dev/dl/)"
	else
		note "Go $have старее, чем требует go.mod ($want): при сборке go сам скачает нужную версию (GOTOOLCHAIN=auto, нужен интернет)"
	fi
}

check_cc() {
	local cc=${CC-}
	if [[ -z $cc ]]; then
		cc=$(cd / && "$GO" env CC 2>/dev/null) || cc=''
	fi
	read -r cc _ <<< "${cc:-gcc}"
	if command -v "$cc" >/dev/null 2>&1; then
		summary+="${summary:+, }$cc"
	else
		problem "не найден компилятор C («$cc») — он нужен Go для cgo"
		need gcc
	fi
}

# parse_tags <теги> — какие бэкенды собираются: want_x11, want_wayland и none. Теги: x11 и/или
# wayland; без них — оба бэкенда; none — заголовков не нашлось (ответ `tags` при ошибке, так его
# передаёт Makefile).
parse_tags() {
	local tag
	local -a list
	want_x11=0
	want_wayland=0
	none=0
	read -ra list <<< "${1//,/ }"
	for tag in "${list[@]}"; do
		case $tag in
		x11) want_x11=1 ;;
		wayland) want_wayland=1 ;;
		none) none=1 ;;
		esac
	done
	if ((!want_x11 && !want_wayland)); then
		want_x11=1
		want_wayland=1
	fi
}

# inspect <теги> заполняет problems, needs, notes и summary.
inspect() {
	local pm
	parse_tags "$1"

	check_go
	check_cc

	if ! command -v "$PKG_CONFIG" >/dev/null 2>&1; then
		problem "не найден pkg-config («$PKG_CONFIG») — через него ищутся OpenGL и заголовки X11/Wayland"
		need pkg-config gl
		# Что из заголовков уже стоит, без pkg-config не узнать — в подсказке набор по умолчанию.
		if ((want_x11)); then need x11; else need wayland; fi
		return 0
	fi
	if ! "$PKG_CONFIG" --exists gl 2>/dev/null; then
		problem "нет заголовков OpenGL (pkg-config не видит gl)"
		need gl
	fi
	if ((none)); then
		problem "нет заголовков ни X11 (pkg-config не видит: $x11_missing), ни Wayland ($wayland_missing) — нужен хотя бы один набор"
		need x11
		pm=$(pkg_manager)
		[[ -z $pm ]] || note "для чистого Wayland без X11 вместо заголовков X11 ставятся: $(pkg_names "$pm" wayland)"
		return 0
	fi
	if ((want_x11)) && [[ -n $x11_missing ]]; then
		problem "нет заголовков X11 (pkg-config не видит: $x11_missing)"
		need x11
	fi
	if ((want_wayland)) && [[ -n $wayland_missing ]]; then
		problem "нет заголовков Wayland (pkg-config не видит: $wayland_missing)"
		need wayland
	fi

	if ((want_x11 && want_wayland)); then
		summary+="${summary:+, }окно — X11 и Wayland"
	elif ((want_x11)); then
		summary+="${summary:+, }окно — только X11"
		pm=$(pkg_manager)
		if [[ -n $wayland_missing && -n $pm ]]; then
			note "в Wayland-сессии окно пойдёт через XWayland; для нативного Wayland поставь $(pkg_names "$pm" wayland) и пересобери"
		fi
		if [[ -n ${WAYLAND_DISPLAY-} && -z ${DISPLAY-} ]]; then
			note "ВНИМАНИЕ: в этой Wayland-сессии нет XWayland (DISPLAY пуст) — окно сборки «только X11» не откроется, нужны заголовки Wayland"
		fi
	else
		summary+="${summary:+, }окно — только Wayland (без X11-библиотек)"
		if [[ ${XDG_SESSION_TYPE-} == x11 ]]; then
			note "ВНИМАНИЕ: сессия сейчас X11 — окно сборки «только Wayland» в ней не откроется, нужны заголовки X11"
		fi
	fi
}

# Список того, чего не хватает, и команда установки — в stderr.
print_problems() {
	local hint
	echo "build-deps.sh: для сборки приложения не хватает:" >&2
	printf '  - %s\n' "${problems[@]}" >&2
	if ((${#needs[@]})); then
		hint=$(install_hint "${needs[@]}")
		if [[ -n $hint ]]; then
			echo "Установи (пакеты сами не ставятся):" >&2
			echo "$hint" >&2
		fi
	fi
	if ((${#notes[@]})); then
		printf '  %s\n' "${notes[@]}" >&2
	fi
}

cmd=${1-}
x11_missing=$(missing_modules "${X11_MODULES[@]}")
wayland_missing=$(missing_modules "${WAYLAND_MODULES[@]}")
# Теги заданы вторым аргументом (пустая строка — оба бэкенда), иначе определяются сами.
if [[ $# -ge 2 ]]; then
	tags=$2
else
	tags=$(auto_tags) || tags=none
fi

case $cmd in
tags)
	if [[ $tags == none ]]; then
		inspect none
		print_problems
		exit 1
	fi
	echo "$tags"
	;;
check)
	inspect "$tags"
	if ((${#problems[@]})); then
		print_problems
		exit 1
	fi
	echo "сборка: $summary"
	if ((${#notes[@]})); then
		printf '  %s\n' "${notes[@]}"
	fi
	;;
report)
	inspect "$tags"
	if ((${#problems[@]})); then printf 'problem\t%s\n' "${problems[@]}"; fi
	if ((${#needs[@]})); then printf 'need\t%s\n' "${needs[@]}"; fi
	printf 'summary\t%s\n' "$summary"
	if ((${#notes[@]})); then printf 'note\t%s\n' "${notes[@]}"; fi
	echo "done"
	;;
cflags)
	parse_tags "$tags"
	modules=()
	if ((want_x11)) && [[ -z $x11_missing ]]; then modules+=("${X11_MODULES[@]}"); fi
	if ((want_wayland)) && [[ -z $wayland_missing ]]; then modules+=("${WAYLAND_MODULES[@]}"); fi
	if ((${#modules[@]})); then
		"$PKG_CONFIG" --cflags-only-I "${modules[@]}"
	fi
	;;
*)
	echo "использование: $0 tags | check [теги] | report [теги] | cflags [теги]" >&2
	exit 2
	;;
esac
