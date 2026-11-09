#!/usr/bin/env bash
# The CI test step. Job logs need a login to read, annotations don't, so this
# publishes what a failure was and how long the slowest test binaries took.
# Usage: scripts/ci_test.sh [log file to summarise instead of running the tests]
set -uo pipefail

# annotate level title text: one GitHub annotation, newlines kept.
annotate() {
	local text=$3
	text=${text//'%'/%25}
	text=${text//$'\r'/%0D}
	text=${text//$'\n'/%0A}
	echo "::$1 title=$2::$text"
}

# heavy are the test binaries closest to the 10-minute -timeout. They run one
# at a time so each has every CPU; the rest run two at a time. The two lists
# are complementary, so no package is left out.
heavy=(rimworld-go/internal/adapters/inbound/ebiten rimworld-go/internal/application/game)

log=${1:-}
code=0
if [ -z "$log" ]; then
	log=$(mktemp)
	# -exec true builds the heavy binaries in parallel without running them, so
	# the -p 1 run below doesn't compile their dependencies one at a time.
	go test -race -exec true "${heavy[@]}" >/dev/null || true
	xvfb-run -a go test -race -p 1 -timeout 10m "${heavy[@]}" 2>&1 | tee "$log"
	code=${PIPESTATUS[0]}
	rest=$(go list ./... | grep -vxF -f <(printf '%s\n' "${heavy[@]}"))
	# shellcheck disable=SC2086 # package paths have no spaces
	xvfb-run -a go test -race -p 2 -timeout 10m $rest 2>&1 | tee -a "$log"
	rc=${PIPESTATUS[0]}
	[ "$code" -ne 0 ] || code=$rc
fi

annotate notice "slowest test binaries" "$(grep -E '^ok' "$log" | awk -F'\t' '{print $3, $2}' | sort -rn | head -8)"
slow=$(grep -E '^ok' "$log" | awk -F'\t' '{t=$3; sub(/s$/, "", t); if (t+0 > 360) print $3, $2}')
if [ -n "$slow" ]; then
	annotate warning "test binaries over 6 of their 10 minutes" "$slow"
fi

if [ "$code" -ne 0 ] || grep -qE '^(FAIL|--- FAIL)' "$log"; then
	annotate error "go test" "$(
		grep -E '^(--- FAIL|    --- FAIL|FAIL|signal: |fatal error)' "$log" | head -40
		grep -E '_test\.go:[0-9]+: ' "$log" | head -30
		grep -A14 -m1 '^panic: ' "$log"
		grep -A32 -m1 'WARNING: DATA RACE' "$log"
		echo "data races: $(grep -c 'WARNING: DATA RACE' "$log"), log lines: $(wc -l <"$log")"
	)"
fi
exit "$code"
