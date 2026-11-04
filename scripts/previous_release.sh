#!/usr/bin/env bash
# Prints the newest tag before the given one that has a published release. A tag
# whose workflow failed has no release, so what it held must still reach players
# in the notes of the next one that does.
#
#   GH_TOKEN=... scripts/previous_release.sh v0.1.25
set -euo pipefail

tag=${1:?usage: previous_release.sh <tag>}
for t in $(git tag --merged "$tag^" --sort=-v:refname 'v*'); do
  if gh release view "$t" --json tagName >/dev/null 2>&1; then
    echo "$t"
    exit 0
  fi
done
