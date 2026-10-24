#!/usr/bin/env python3
"""
Конвертер стратегий Flowseal (general*.bat для winws.exe) в аргументы nfqws для Linux,
урезанные только под YouTube и Discord.

  python3 -I convert.py <папка flowseal> <папка установки>

Что делает:
  * lists/youtube.txt  — из list-google.txt (всё, кроме discord-доменов)
  * lists/discord.txt  — только discord-домены из list-general.txt (+ из list-google)
  * lists/exclude.txt, lists/ipset-exclude.txt — исключения как у Flowseal
  * strategies/<имя>.args — аргументы nfqws, по одному на строку
  * strategies/<имя>.env  — TCP/UDP порты для nftables
  * strategies/index.txt  — "<имя>\t<оригинальное имя .bat>"

Профили, завязанные на ipset-all (весь трафик Cloudflare и т.п.) и GameFilter, выбрасываются:
нам нужны только YouTube и Discord.
"""
import os
import re
import shlex
import sys


def read_lines(path):
    with open(path, encoding="utf-8", errors="replace") as f:
        out = []
        for line in f:
            line = line.strip().lstrip("﻿")
            if line and not line.startswith("#"):
                out.append(line)
        return out


def is_discord(domain):
    d = domain.lstrip("^").lower()
    return "discord" in d or d == "dis.gd"


def build_lists(src_lists, dst_lists):
    google = read_lines(os.path.join(src_lists, "list-google.txt"))
    general = read_lines(os.path.join(src_lists, "list-general.txt"))

    youtube = [d for d in google if not is_discord(d)]
    discord = [d for d in general if is_discord(d)] + [d for d in google if is_discord(d)]
    discord = list(dict.fromkeys(discord))  # без дублей, порядок сохраняем

    if not youtube or not discord:
        sys.exit("ошибка: не удалось собрать списки доменов (формат репозитория Flowseal изменился?)")

    os.makedirs(dst_lists, exist_ok=True)
    files = {
        "youtube.txt": youtube,
        "discord.txt": discord,
        "exclude.txt": read_lines(os.path.join(src_lists, "list-exclude.txt")),
        "ipset-exclude.txt": read_lines(os.path.join(src_lists, "ipset-exclude.txt")),
    }
    for name, lines in files.items():
        with open(os.path.join(dst_lists, name), "w", encoding="utf-8") as f:
            f.write("\n".join(lines) + "\n")


def slug_for(bat_name):
    stem = bat_name[:-4]  # без .bat
    m = re.fullmatch(r"general\s*\((.+)\)", stem, re.I)
    if stem.lower() == "general":
        return "general"
    if not m:
        return None
    s = re.sub(r"[^a-z0-9]+", "-", m.group(1).lower()).strip("-")
    return s or None


def extract_winws_args(text):
    # склеиваем продолжения строк cmd (^ в конце строки)
    text = re.sub(r"\^\s*\r?\n", " ", text)
    for line in text.splitlines():
        m = re.search(r'winws\.exe"?\s+(.*)$', line, re.I)
        if m:
            return m.group(1)
    return None


def convert_bat(path, base):
    with open(path, encoding="utf-8", errors="replace") as f:
        raw = extract_winws_args(f.read())
    if raw is None:
        return None

    bin_dir = os.path.join(base, "fake") + "/"
    lists_dir = os.path.join(base, "lists") + "/"
    raw = raw.replace("%BIN%", bin_dir).replace("%LISTS%", lists_dir)
    raw = raw.replace("^!", "!")

    tokens = shlex.split(raw, posix=True)

    tcp_ports, udp_ports = "", ""
    profiles, cur = [], []
    for t in tokens:
        if t == "--new":
            profiles.append(cur)
            cur = []
            continue
        if t.startswith("--wf-tcp="):
            tcp_ports = t.split("=", 1)[1]
            continue
        if t.startswith("--wf-udp="):
            udp_ports = t.split("=", 1)[1]
            continue
        if t.startswith("--wf-"):  # прочие опции WinDivert на Linux не нужны
            continue
        cur.append(t)
    if cur:
        profiles.append(cur)

    def clean_ports(p):
        parts = [x for x in p.split(",") if x and "%" not in x]
        return ",".join(parts)

    tcp_ports, udp_ports = clean_ports(tcp_ports), clean_ports(udp_ports)

    rename = {
        lists_dir + "list-general.txt": lists_dir + "discord.txt",
        lists_dir + "list-google.txt": lists_dir + "youtube.txt",
        lists_dir + "list-exclude.txt": lists_dir + "exclude.txt",
        lists_dir + "ipset-exclude.txt": lists_dir + "ipset-exclude.txt",
    }

    kept = []
    for prof in profiles:
        joined = " ".join(prof)
        if "ipset-all" in joined or "%GameFilter" in joined:
            continue
        out = []
        for t in prof:
            if "-user.txt" in t:
                continue
            if "=" in t:
                k, v = t.split("=", 1)
                v = rename.get(v, v)
                t = k + "=" + v
            out.append(t)
        kept.append(out)

    if not kept:
        return None

    args = []
    for i, prof in enumerate(kept):
        if i:
            args.append("--new")
        args.extend(prof)
    return args, tcp_ports, udp_ports


def main():
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    src, base = sys.argv[1], sys.argv[2]

    build_lists(os.path.join(src, "lists"), os.path.join(base, "lists"))

    sdir = os.path.join(base, "strategies")
    os.makedirs(sdir, exist_ok=True)
    for f in os.listdir(sdir):
        os.remove(os.path.join(sdir, f))

    index = []
    for name in sorted(os.listdir(src), key=lambda n: (n.lower() != "general.bat", n.lower())):
        if not (name.lower().startswith("general") and name.lower().endswith(".bat")):
            continue
        slug = slug_for(name)
        if not slug:
            continue
        res = convert_bat(os.path.join(src, name), base)
        if not res:
            print(f"  пропущено: {name}")
            continue
        args, tcp, udp = res
        with open(os.path.join(sdir, slug + ".args"), "w", encoding="utf-8") as f:
            f.write("\n".join(args) + "\n")
        with open(os.path.join(sdir, slug + ".env"), "w", encoding="utf-8") as f:
            f.write(f"TCP_PORTS={tcp}\nUDP_PORTS={udp}\n")
        index.append(f"{slug}\t{name[:-4]}")
        print(f"  {name}  ->  {slug}")

    if not index:
        sys.exit("ошибка: не найдено ни одной стратегии general*.bat")
    with open(os.path.join(sdir, "index.txt"), "w", encoding="utf-8") as f:
        f.write("\n".join(index) + "\n")


if __name__ == "__main__":
    main()
