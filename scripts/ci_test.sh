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

log=${1:-}
code=0
if [ -z "$log" ]; then
	log=$(mktemp)
	xvfb-run -a go test -race -p 2 -timeout 10m ./... 2>&1 | tee "$log"
	code=${PIPESTATUS[0]}
fi

annotate notice "slowest test binaries" "$(grep -E '^ok' "$log" | awk -F'\t' '{print $3, $2}' | sort -rn | head -8)"

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
