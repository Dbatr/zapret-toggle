#!/bin/bash
# Прогоняет `nfqws --dry-run` по всем стратегиям из <база>/strategies/index.txt.
# Не прошедшие проверку убираются из index.txt (и их .args/.env удаляются) с предупреждением.
#
#   check-strategies.sh <база, напр. /opt/zapret-dy> <путь к nfqws>
#
# Выход: 0 — осталась хотя бы одна стратегия, 1 — не прошла ни одна.
set -euo pipefail

if [[ $# -ne 2 ]]; then
	echo "использование: $0 <база> <nfqws>" >&2
	exit 2
fi
base=$1
nfqws=$2
sdir=$base/strategies
index=$sdir/index.txt

[[ -f $index ]] || { echo "check-strategies: нет $index" >&2; exit 1; }
[[ -x $nfqws ]] || { echo "check-strategies: nfqws не найден или не исполняемый: $nfqws" >&2; exit 1; }

kept=()
failed=0
while IFS=$'\t' read -r slug name || [[ -n $slug ]]; do
	[[ -n $slug ]] || continue
	ok=1
	if [[ ! $slug =~ ^[a-z0-9-]+$ || ! -f $sdir/$slug.args || ! -f $sdir/$slug.env ]]; then
		ok=0
		out="нет файлов стратегии или недопустимое имя"
	else
		mapfile -t args < "$sdir/$slug.args"
		# Те же фиксированные опции, что в run.sh.
		if ! out=$("$nfqws" --dry-run --qnum=200 --dpi-desync-fwmark=0x40000000 "${args[@]}" 2>&1); then
			ok=0
		fi
	fi
	if ((ok)); then
		kept+=("$slug"$'\t'"$name")
	else
		failed=$((failed + 1))
		echo "ПРЕДУПРЕЖДЕНИЕ: стратегия '$name' ($slug) не прошла nfqws --dry-run и исключена:" >&2
		tail -n 3 <<<"$out" | sed 's/^/    /' >&2
		rm -f "$sdir/$slug.args" "$sdir/$slug.env"
	fi
done < "$index"

if ((${#kept[@]} == 0)); then
	echo "check-strategies: ни одна стратегия не прошла проверку" >&2
	exit 1
fi

printf '%s\n' "${kept[@]}" > "$index.new"
mv -f "$index.new" "$index"
echo "check-strategies: проверено стратегий: $((${#kept[@]} + failed)), рабочих: ${#kept[@]}, исключено: $failed"
