#!/bin/bash
# Обновление стратегий Flowseal (github.com/Flowseal/zapret-discord-youtube):
#   1. скачивает последний коммит в third_party/flowseal (general*.bat, lists/, bin/*.bin → fake/);
#   2. прогоняет convert.py и nfqws --dry-run по всем стратегиям во временном каталоге
#      (не прошедшие — исключаются с предупреждением; если не прошла ни одна — third_party не меняется);
#   3. обновляет COMMIT и third_party/README.md;
#   4. переустанавливает стратегии в /opt/zapret-dy (sudo scripts/install.sh --strategies-only).
#
#   scripts/update-flowseal.sh [--force] [--no-install]
#     --force       обновить, даже если коммит не изменился
#     --no-install  только обновить third_party, в /opt не ставить
set -euo pipefail

REPO=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
GH_REPO=Flowseal/zapret-discord-youtube
DEST=$REPO/third_party/flowseal
NFQWS=$REPO/third_party/zapret/nfqws

die() {
	echo "update-flowseal.sh: ошибка: $*" >&2
	exit 1
}
step() { echo "==> $*"; }

force=0
do_install=1
for arg in "$@"; do
	case $arg in
	--force) force=1 ;;
	--no-install) do_install=0 ;;
	*) die "неизвестный аргумент: $arg" ;;
	esac
done

[[ $EUID -ne 0 ]] || die "запускай от обычного пользователя: sudo понадобится только на шаге установки и будет запрошен сам"
for cmd in curl tar python3; do
	command -v "$cmd" >/dev/null || die "не найдена команда '$cmd'"
done
[[ -x $NFQWS ]] || die "нет $NFQWS"

work=$(mktemp -d)
staged=$REPO/third_party/.flowseal.new
cleanup() { rm -rf "$work" "$staged"; }
trap cleanup EXIT

step "Последний коммит $GH_REPO"
sha=$(curl -fsSL -H 'Accept: application/vnd.github+json' "https://api.github.com/repos/$GH_REPO/commits/HEAD" |
	python3 -I -c 'import json, sys; print(json.load(sys.stdin)["sha"])')
[[ $sha =~ ^[0-9a-f]{40}$ ]] || die "не удалось получить хэш коммита (ответ: '$sha')"
old=$(cat "$DEST/COMMIT" 2>/dev/null || true)
echo "    сейчас: ${old:-нет}"
echo "    новый:  $sha"
if [[ $sha == "$old" ]] && ((!force)); then
	echo "Уже актуально. Принудительно: $0 --force"
	exit 0
fi

step "Скачивание"
curl -fsSL "https://codeload.github.com/$GH_REPO/tar.gz/$sha" -o "$work/src.tar.gz"
mkdir "$work/src"
tar -xzf "$work/src.tar.gz" -C "$work/src" --strip-components=1 --no-same-owner --no-same-permissions
src=$work/src

for f in list-general.txt list-google.txt list-exclude.txt ipset-exclude.txt; do
	[[ -f $src/lists/$f ]] || die "в репозитории нет lists/$f — структура Flowseal изменилась"
done
compgen -G "$src/general*.bat" >/dev/null || die "в репозитории нет general*.bat"
compgen -G "$src/bin/*.bin" >/dev/null || die "в репозитории нет bin/*.bin"

# Только нужное: стратегии, списки и фейковые пакеты (winws.exe и прочее для Windows не берём).
rm -rf "$staged"
mkdir -p "$staged/lists" "$staged/fake"
cp -- "$src"/general*.bat "$staged/"
cp -- "$src"/lists/*.txt "$staged/lists/"
cp -- "$src"/bin/*.bin "$staged/fake/"
echo "$sha" > "$staged/COMMIT"
chmod -R u=rwX,go=rX "$staged"

step "Проверка: convert.py + nfqws --dry-run"
# Как при установке: каталог, читаемый всеми (nfqws под root сбрасывает права).
test_base=$work/zapret-dy-test
mkdir -p "$test_base/fake"
cp -- "$staged"/fake/*.bin "$test_base/fake/"
python3 -I "$REPO/system/convert.py" "$staged" "$test_base"
chmod -R a+rX "$work"
"$REPO/scripts/check-strategies.sh" "$test_base" "$NFQWS" ||
	die "ни одна стратегия не прошла проверку — third_party/flowseal не изменён"

step "Обновление third_party/flowseal"
rm -rf "$DEST.old"
if [[ -d $DEST ]]; then
	mv "$DEST" "$DEST.old"
fi
if ! mv "$staged" "$DEST"; then
	[[ -d $DEST.old ]] && mv "$DEST.old" "$DEST"
	die "не удалось заменить $DEST"
fi
rm -rf "$DEST.old"

readme=$REPO/third_party/README.md
if [[ -f $readme ]]; then
	sed -i -E "s/^(- Коммит: \`)[0-9a-f]{40}(\`.*)$/\1$sha\2/" "$readme"
	grep -q "$sha" "$readme" || echo "ПРЕДУПРЕЖДЕНИЕ: не нашёл строку коммита в $readme — обнови вручную" >&2
fi

echo "Flowseal обновлён до ${sha:0:12}. Изменения: git -C $(printf '%q' "$REPO") status third_party"

if ((!do_install)); then
	echo "Установка пропущена (--no-install). Поставить: sudo scripts/install.sh --strategies-only"
	exit 0
fi
if [[ ! -x /opt/zapret-dy/nfqws ]]; then
	echo "zapret-dy ещё не установлен — запусти: sudo scripts/install.sh"
	exit 0
fi
step "Переустановка стратегий в /opt/zapret-dy (нужен sudo)"
sudo "$REPO/scripts/install.sh" --strategies-only
