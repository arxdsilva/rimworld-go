#!/usr/bin/env python3
"""Add the GPL-3.0 short header to the project's own Go files.

Usage: python3 scripts/spdx_headers.py [--check]

The header is two comment lines and a blank line, above any build
constraint or package comment:

    // SPDX-License-Identifier: GPL-3.0-only
    // Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

Skipped: third_party/, pkg/spritenormals/ (a port of MIT code, which keeps
its own notice), testdata/, generated files and files that already carry
an SPDX line. --check lists the files that lack the header and exits 1.
"""

import pathlib
import re
import subprocess
import sys

HEADER = (
    "// SPDX-License-Identifier: GPL-3.0-only\n"
    "// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva\n"
    "\n"
)
SKIP_PREFIXES = ("third_party/", "pkg/spritenormals/")
GENERATED = re.compile(r"^// Code generated .* DO NOT EDIT\.$", re.M)


def own_go_files(root):
    out = subprocess.run(["git", "ls-files", "*.go"], cwd=root, check=True,
                         capture_output=True, text=True).stdout.split()
    for rel in out:
        if rel.startswith(SKIP_PREFIXES) or "/testdata/" in rel:
            continue
        yield rel


def main():
    check = "--check" in sys.argv[1:]
    root = pathlib.Path(subprocess.run(["git", "rev-parse", "--show-toplevel"], check=True,
                                       capture_output=True, text=True).stdout.strip())
    missing = []
    for rel in own_go_files(root):
        path = root / rel
        src = path.read_text(encoding="utf-8")
        if "SPDX-License-Identifier" in src[:400] or GENERATED.search(src):
            continue
        missing.append(rel)
        if not check:
            path.write_text(HEADER + src, encoding="utf-8")
    if check:
        for rel in missing:
            print(rel)
        return 1 if missing else 0
    print(f"added the header to {len(missing)} files")
    return 0


if __name__ == "__main__":
    sys.exit(main())
