// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package skill

// LearnLog records today's skill XP and the seven completed/current daily
// buckets. The owner advances it by absolute game day, never wall time.
type LearnLog struct {
	Day   int32
	Today [NumSkills]int32
	Week  [7][NumSkills]int32
	Carry [NumSkills]int16
}

// Roll clears the current day and any elapsed weekday buckets. The
// backward/unchanged case is intentional for paused or restored sessions.
func (l *LearnLog) Roll(day int32) {
	if l.Day == day {
		return
	}
	for d := l.Day + 1; d <= day && d <= l.Day+7; d++ {
		l.Week[d%7] = [NumSkills]int32{}
	}
	l.Today, l.Day = [NumSkills]int32{}, day
}

// Saturated is the per-skill learning cap reached during day.
func (l *LearnLog) Saturated(day, capDay int32) [NumSkills]bool {
	var out [NumSkills]bool
	if l.Day != day {
		return out
	}
	for sk := range out {
		out[sk] = l.Today[sk] >= capDay
	}
	return out
}

// LastWeek sums the preceding seven day buckets in skill ID order.
func (l *LearnLog) LastWeek(day int32) [NumSkills]int {
	var out [NumSkills]int
	for d := day - 6; d <= day; d++ {
		if d < 0 || d > l.Day || l.Day-d >= 7 {
			continue
		}
		for sk := range out {
			out[sk] += int(l.Week[d%7][sk])
		}
	}
	return out
}
