#!/bin/bash
# Прогоняет `nfqws --dry-run` по всем стратегиям из <база>/strategies/index.txt.
# Не прошедшие проверку убираются из index.txt (и их .args/.env удаляются) с предупреждением.
#
# Аргументы приходят из .bat стороннего репозитория Flowseal, а nfqws запускается от root,
# поэтому до --dry-run каждая опция сверяется с белым списком: никаких --pidfile, --debug=@файл,
# --uid/--user, --hostlist-auto (пишут файлы или не сбрасывают права), своих --qnum
# и --dpi-desync-fwmark; файлы — только из <база>/fake и <база>/lists. Имена опций — только
# полные: getopt_long в nfqws принимает и сокращения (--pidf=...).
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

# Опции nfqws, которые только настраивают обход DPI и читают файлы из <база>.
allowed=" new skip filter-l3 filter-tcp filter-udp filter-l7
	ipset ipset-ip ipset-exclude ipset-exclude-ip
	hostlist hostlist-domains hostlist-exclude hostlist-exclude-domains
	ctrack-timeouts ctrack-disable ipcache-lifetime ipcache-hostname
	wsize wssize wssize-cutoff wssize-forced-cutoff synack-split
	orig-ttl orig-ttl6 orig-autottl orig-autottl6 orig-tcp-flags-set orig-tcp-flags-unset
	orig-mod-start orig-mod-cutoff
	dup dup-replace dup-ttl dup-ttl6 dup-autottl dup-autottl6 dup-tcp-flags-set dup-tcp-flags-unset
	dup-fooling dup-ts-increment dup-badseq-increment dup-badack-increment dup-ip-id dup-start dup-cutoff
	hostcase hostspell hostnospace domcase methodeol ip-id
	dpi-desync dpi-desync-ttl dpi-desync-ttl6 dpi-desync-autottl dpi-desync-autottl6
	dpi-desync-tcp-flags-set dpi-desync-tcp-flags-unset dpi-desync-fooling dpi-desync-repeats
	dpi-desync-skip-nosni dpi-desync-split-pos dpi-desync-split-seqovl dpi-desync-split-seqovl-pattern
	dpi-desync-fakedsplit-pattern dpi-desync-fakedsplit-mod dpi-desync-hostfakesplit-midhost
	dpi-desync-hostfakesplit-mod dpi-desync-ipfrag-pos-udp dpi-desync-ipfrag-pos-tcp
	dpi-desync-ts-increment dpi-desync-badseq-increment dpi-desync-badack-increment
	dpi-desync-any-protocol dpi-desync-fake-tcp-mod dpi-desync-fake-http dpi-desync-fake-tls
	dpi-desync-fake-tls-mod dpi-desync-fake-unknown dpi-desync-fake-syndata dpi-desync-fake-quic
	dpi-desync-fake-wireguard dpi-desync-fake-dht dpi-desync-fake-discord dpi-desync-fake-stun
	dpi-desync-fake-unknown-udp dpi-desync-udplen-increment dpi-desync-udplen-pattern
	dpi-desync-start dpi-desync-cutoff "
allowed=${allowed//[$'\t\n']/ }

# Проверяет массив args; при нарушении печатает причину и возвращает 1.
check_args() {
	local a name val path
	for a in "${args[@]}"; do
		if [[ ! $a =~ ^--([a-z0-9-]+)(=(.*))?$ ]]; then
			echo "недопустимый аргумент: '$a'"
			return 1
		fi
		name=${BASH_REMATCH[1]}
		val=${BASH_REMATCH[3]}
		if [[ $allowed != *" $name "* ]]; then
			echo "опция --$name не разрешена"
			return 1
		fi
		[[ $val == */* ]] || continue
		# Файл: "<путь>", "@<путь>" или "+<смещение>@<путь>".
		path=$val
		if [[ $path =~ ^(\+[0-9]+)?@(.*)$ ]]; then
			path=${BASH_REMATCH[2]}
		fi
		case $path in
		"$base"/fake/* | "$base"/lists/*) ;;
		*)
			echo "--$name: файл вне $base/fake и $base/lists: '$val'"
			return 1
			;;
		esac
		# Только имя файла прямо в каталоге: без подкаталогов и "..".
		if [[ ! ${path#"$base"/*/} =~ ^[A-Za-z0-9_][A-Za-z0-9._-]*$ ]]; then
			echo "--$name: недопустимый путь: '$val'"
			return 1
		fi
	done
}

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
		if ! out=$(check_args); then
			ok=0
		# Те же фиксированные опции, что в run.sh.
		elif ! out=$("$nfqws" --dry-run --qnum=200 --dpi-desync-fwmark=0x40000000 "${args[@]}" 2>&1); then
			ok=0
		fi
	fi
	if ((ok)); then
		kept+=("$slug"$'\t'"$name")
	else
		failed=$((failed + 1))
		echo "ПРЕДУПРЕЖДЕНИЕ: стратегия '$name' ($slug) не прошла проверку и исключена:" >&2
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
