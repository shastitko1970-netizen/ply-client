#!/usr/bin/env python3
"""Fail unless each Windows exe has an embedded icon group.

Shortcuts use IconLocation exe,0. The tray calls ExtractIconW. The window
and taskbar use the same PE resource. An exe built without rsrc has an empty
resource directory and Windows shows a blank icon.
"""

from __future__ import annotations

import struct
import sys
from pathlib import Path

RT_ICON = 3
RT_GROUP_ICON = 14


def resource_type_ids(data: bytes) -> list[int]:
    if data[:2] != b"MZ":
        raise ValueError("not a PE file")
    e = struct.unpack_from("<I", data, 0x3C)[0]
    if data[e : e + 4] != b"PE\0\0":
        raise ValueError("missing PE signature")
    _, nsec, _, _, _, optsz, _ = struct.unpack_from("<HHIIIHH", data, e + 4)
    opt = e + 24
    magic = struct.unpack_from("<H", data, opt)[0]
    if magic == 0x20B:
        num_dd = struct.unpack_from("<I", data, opt + 108)[0]
        dd_off = opt + 112
    elif magic == 0x10B:
        num_dd = struct.unpack_from("<I", data, opt + 92)[0]
        dd_off = opt + 96
    else:
        raise ValueError(f"unknown optional header {magic:#x}")
    if num_dd < 3:
        return []
    rva, size = struct.unpack_from("<II", data, dd_off + 8 * 2)
    if rva == 0 or size == 0:
        return []
    sec_off = opt + optsz
    sections = []
    for i in range(nsec):
        o = sec_off + i * 40
        vsz, va, rsz, raw = struct.unpack_from("<IIII", data, o + 8)
        sections.append((va, max(vsz, rsz), raw))

    def rva_to_off(addr: int) -> int:
        for va, span, raw in sections:
            if va <= addr < va + span:
                return raw + (addr - va)
        raise ValueError(f"resource RVA {addr:#x} is outside the sections")

    off = rva_to_off(rva)
    named, ids = struct.unpack_from("<HH", data, off + 12)
    types = []
    base = off + 16
    for i in range(named + ids):
        eid, _ = struct.unpack_from("<II", data, base + i * 8)
        if eid & 0x80000000:
            continue
        types.append(eid)
    return types


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print("usage: check_pe_icon.py EXE [EXE...]", file=sys.stderr)
        return 2
    failed = False
    for arg in argv[1:]:
        path = Path(arg)
        try:
            types = resource_type_ids(path.read_bytes())
        except (OSError, ValueError, struct.error) as err:
            print(f"{path}: {err}", file=sys.stderr)
            failed = True
            continue
        if RT_ICON not in types or RT_GROUP_ICON not in types:
            print(
                f"{path}: missing icon resource (types={types or 'empty'})",
                file=sys.stderr,
            )
            failed = True
            continue
        print(f"{path}: icon resource ok")
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
