#!/bin/bash
# Правила nftables для zapret-dy: своя таблица inet zapretdy, с firewalld не пересекается.
#
#   firewall.sh up <slug>   — (пере)создать таблицу с портами из strategies/<slug>.env
#   firewall.sh down        — удалить таблицу (без ошибки, если её нет)
#
# Схема правил — из мануала zapret (nftables, nfqws, очередь 200, fwmark 0x40000000).
set -euo pipefail

BASE=/opt/zapret-dy
TABLE="inet zapretdy"
QNUM=200
MARK=0x40000000

die() {
	echo "firewall.sh: $*" >&2
	exit 2
}

# Читает KEY=VALUE из .env без source: файл генерируется конвертером, но выполнять его не нужно.
env_value() {
	local file=$1 key=$2 line
	while IFS= read -r line || [[ -n $line ]]; do
		if [[ $line == "$key="* ]]; then
			printf '%s' "${line#*=}"
			return 0
		fi
	done < "$file"
}

# "80,443,19294-19344" -> "80, 443, 19294-19344" (элементы анонимного множества nft)
nft_set() {
	local p=$1
	[[ $p =~ ^[0-9]+(-[0-9]+)?(,[0-9]+(-[0-9]+)?)*$ ]] || die "некорректный список портов: '$p'"
	printf '%s' "${p//,/, }"
}

rules() {
	local tcp=$1 udp=$2
	cat <<EOF
table $TABLE
delete table $TABLE
table $TABLE {
	chain post {
		type filter hook postrouting priority srcnat + 1; policy accept;
EOF
	if [[ -n $tcp ]]; then
		cat <<EOF
		oifname != "lo" meta mark and $MARK == 0 tcp dport { $tcp } ct original packets 1-6 queue num $QNUM bypass
		oifname != "lo" meta mark and $MARK == 0 tcp dport { $tcp } tcp flags fin,rst queue num $QNUM bypass
EOF
	fi
	if [[ -n $udp ]]; then
		cat <<EOF
		oifname != "lo" meta mark and $MARK == 0 udp dport { $udp } ct original packets 1-6 queue num $QNUM bypass
EOF
	fi
	cat <<EOF
	}
	chain pre {
		type filter hook prerouting priority filter; policy accept;
EOF
	if [[ -n $tcp ]]; then
		cat <<EOF
		iifname != "lo" tcp sport { $tcp } ct reply packets 1-3 queue num $QNUM bypass
		iifname != "lo" tcp sport { $tcp } tcp flags & (syn | ack) == (syn | ack) queue num $QNUM bypass
		iifname != "lo" tcp sport { $tcp } tcp flags fin,rst queue num $QNUM bypass
EOF
	fi
	cat <<EOF
	}
	chain predefrag {
		type filter hook output priority -401; policy accept;
		meta mark and $MARK != 0 notrack
	}
}
EOF
}

# Заполняет TCP и UDP (уже в синтаксисе nft) из strategies/<slug>.env.
load_ports() {
	local slug=$1 envfile
	[[ $slug =~ ^[a-z0-9-]+$ ]] || die "недопустимое имя стратегии: '$slug'"
	envfile=$BASE/strategies/$slug.env
	[[ -f $envfile ]] || die "нет файла портов: $envfile"

	TCP=$(env_value "$envfile" TCP_PORTS)
	UDP=$(env_value "$envfile" UDP_PORTS)
	[[ -n $TCP || -n $UDP ]] || die "в $envfile не заданы ни TCP_PORTS, ни UDP_PORTS"
	if [[ -n $TCP ]]; then TCP=$(nft_set "$TCP"); fi
	if [[ -n $UDP ]]; then UDP=$(nft_set "$UDP"); fi
}

up() {
	load_ports "$1"
	# Один скрипт = одна транзакция: либо вся таблица применилась, либо ничего.
	rules "$TCP" "$UDP" | nft -f -
}

down() {
	# "table X" перед "delete table X" — чтобы удаление не падало, если таблицы нет.
	printf 'table %s\ndelete table %s\n' "$TABLE" "$TABLE" | nft -f -
}

case ${1-} in
up) up "${2-}" ;;
down) down ;;
print)
	# Отладка: показать сгенерированный скрипт, ничего не применяя.
	load_ports "${2-}"
	rules "$TCP" "$UDP"
	;;
*)
	echo "использование: $0 up <slug> | down | print <slug>" >&2
	exit 2
	;;
esac
