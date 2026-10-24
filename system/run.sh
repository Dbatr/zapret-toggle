#!/bin/bash
# Запуск nfqws в foreground со стратегией <slug> (вызывается из zapret-dy@.service).
set -euo pipefail

BASE=/opt/zapret-dy
slug=${1-}

if [[ ! $slug =~ ^[a-z0-9-]+$ ]]; then
	echo "run.sh: недопустимое имя стратегии: '$slug'" >&2
	exit 2
fi
argsfile=$BASE/strategies/$slug.args
if [[ ! -f $argsfile ]]; then
	echo "run.sh: стратегия не найдена: $argsfile" >&2
	exit 2
fi

mapfile -t args < "$argsfile"
exec "$BASE/nfqws" --qnum=200 --dpi-desync-fwmark=0x40000000 "${args[@]}"
