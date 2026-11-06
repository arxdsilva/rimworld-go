#!/usr/bin/env python3
"""Regenerate .agents/handoffs/INDEX.md from the ledger headings.

Run from the repo root after appending a ledger section:
    python3 scripts/handoff_index.py
"""
import os, re
H = ".agents/handoffs"
rows, fams = [], []
for f in sorted(os.listdir(H)):
    if not re.fullmatch(r"[A-Z]+\.md", f) or f == "INDEX.md":
        continue
    lines = open(os.path.join(H, f), encoding="utf-8").read().split("\n")
    for ln in lines:
        if ln.startswith("Families:"):
            fams.append((f, ln[len("Families:"):].split(". ")[0].strip()))
            break
    for i, ln in enumerate(lines):
        if ln.startswith("## "):
            stem = ln[3:].strip()
            nxt = next((l for l in lines[i + 1:] if l.strip()), "")
            m = re.match(r"(.*) · (\d{4}-\d{2}-\d{2}) · (.*)$", nxt)
            title, date = (m.group(1).strip(), m.group(2)) if m else ("", "")
            title = title.replace("|", "\\|")
            rows.append((stem, title, date, f))
def key(r):
    return [int(t) if t.isdigit() else t for t in re.split(r"(\d+)", r[0])]
rows.sort(key=key)
out = ["# Handoff index", "",
       "One row per handoff section. Open the ledger at `#<lowercased ID>`. A new task appends its section to the ledger of its family (below; start a new ledger only when none fits, never named `AGENTS.md`) and adds its row here: `python3 scripts/handoff_index.py` rebuilds this file. Format: `.agents/templates/HANDOFF.md`.", "",
       "| Ledger | Families |", "|---|---|"]
out += [f"| [{f}]({f}) | {fa} |" for f, fa in fams]
out += ["", "| ID | Title | Date | Ledger |", "|---|---|---|---|"]
out += [f"| {s} | {t} | {d} | [{f}]({f}#{s.lower()}) |" for s, t, d, f in rows]
open(os.path.join(H, "INDEX.md"), "w", encoding="utf-8").write("\n".join(out) + "\n")
print(len(rows), "rows,", len(fams), "ledgers")
