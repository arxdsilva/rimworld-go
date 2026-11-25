// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package animal

// RateSpan apportions production over the last span ticks.
func RateSpan(now uint64, per, period, span int) int {
	from := uint64(0)
	if now > uint64(span) {
		from = now - uint64(span)
	}
	p, q := uint64(per), uint64(period)
	return int(now*p/q - from*p/q)
}

// NextDecision is the next deterministic decision tick for animal id.
func NextDecision(id, now uint64) uint64 {
	return now + DecideEvery - (now+id)%DecideEvery
}
