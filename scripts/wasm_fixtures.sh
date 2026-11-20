#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-only
# Copyright (C) 2026 Arthur Rodrigues Xavier da Silva
#
# Rebuild the Wasm test fixtures of internal/adapters/outbound/wasmhost
# (PLG-063) from testdata/rust/<name>/ and copy <name>.wasm to testdata/.
# Needs the pinned toolchain in testdata/rust/TOOLCHAIN with the
# wasm32-unknown-unknown target; refuses any other rustc, so the committed
# bytes stay reproducible. OUT_DIR overrides where the .wasm files go (the
# fixture test rebuilds into a temp dir and compares). The cargo target dir
# is always a temp dir, never the repo.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
src="$root/internal/adapters/outbound/wasmhost/testdata"
out="${OUT_DIR:-$src}"

want="$(cat "$src/rust/TOOLCHAIN")"
have="$(rustc -V)"
if [ "$have" != "$want" ]; then
	echo "wasm_fixtures: need '$want', have '$have'" >&2
	exit 1
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
export CARGO_TARGET_DIR="$tmp/target"
mkdir -p "$out"

for dir in "$src"/rust/*/; do
	name="$(basename "$dir")"
	(
		cd "$dir"
		cargo rustc --release --locked --offline --target wasm32-unknown-unknown -- \
			--remap-path-prefix "$HOME=/home" \
			--remap-path-prefix "$dir=/fixture/$name" \
			-C link-arg=--max-memory=16777216 \
			-C link-arg=--strip-all \
			-C link-arg=-zstack-size=65536
	) >&2
	cp "$CARGO_TARGET_DIR/wasm32-unknown-unknown/release/$name.wasm" "$out/$name.wasm"
done
