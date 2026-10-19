"""Loads assets/palettes/materials.yaml (flow-map subset of YAML only)."""
import os
import re

from . import contracts

_ENTRY = re.compile(r'^\s{2}([a-z_]+):\s*\{(.*)\}\s*$')
_EMIT = re.compile(r'^\s{2}([a-z_]+):\s*"(#[0-9a-fA-F]{6})"\s*$')
_KV = re.compile(r'([a-z_]+):\s*("?)([^",]+)\2')


def hex_to_linear(h):
    h = h.lstrip("#")
    out = []
    for i in (0, 2, 4):
        c = int(h[i:i + 2], 16) / 255.0
        out.append(c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4)
    return tuple(out)


def hex_to_srgb(h):
    h = h.lstrip("#")
    return tuple(int(h[i:i + 2], 16) / 255.0 for i in (0, 2, 4))


def load(path=None):
    path = path or os.path.join(contracts.PALETTE_DIR, "materials.yaml")
    mats, emis, section = {}, {}, None
    with open(path, encoding="utf-8") as f:
        for line in f:
            if line.startswith("materials:"):
                section = "m"
                continue
            if line.startswith("emissive:"):
                section = "e"
                continue
            if section == "m":
                m = _ENTRY.match(line)
                if m:
                    kv = {k: v for k, _, v in _KV.findall(m.group(2))}
                    mats[m.group(1)] = {
                        "light": kv["light"], "base": kv["base"], "shadow": kv["shadow"],
                        "tint": kv.get("tint", "false").strip() == "true",
                    }
            elif section == "e":
                m = _EMIT.match(line)
                if m:
                    emis[m.group(1)] = m.group(2)
    if not mats:
        raise RuntimeError("no materials parsed from %s" % path)
    return mats, emis
