#!/usr/bin/env bash
# Prints the commit messages since the previous release as release notes, one
# collapsed section per commit that has a body. GoReleaser's changelog lists
# subjects only; this is the rest of what each change says.
#
#   scripts/release_details.sh v0.1.19 [v0.1.18] > details.md
#
# Without the second argument the range starts at the tag before, released or not.
set -euo pipefail

tag=${1:?usage: release_details.sh <tag>}
range=$tag
prev=${2:-$(git describe --tags --abbrev=0 "$tag^" 2>/dev/null || true)}
if [ -n "$prev" ]; then
  range=$prev..$tag
fi

printf '## Details\n'
for sha in $(git log --reverse --no-merges --format=%h "$range"); do
  body=$(git log -1 --format=%b "$sha")
  [ -n "$body" ] || continue
  printf '\n<details>\n<summary><code>%s</code> %s</summary>\n\n%s\n\n</details>\n' \
    "$sha" "$(git log -1 --format=%s "$sha")" "$body"
done
