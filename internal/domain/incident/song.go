// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package incident

import "rimworld-go/internal/domain/sim/clock"

// The sky song (25.2, #13): an original timed colony-wide positive mood
// event. It plays from the tick it starts until State.SongUntil, 1.75 to
// 2.25 days; while it plays every colonist holds the mood.SkySong state
// thought. Its 20-day cooldown is far longer than the song itself.

// SongMin and SongMax bound the song's length, in hundredths of a day.
const SongMin, SongMax = 175, 225

// SongTicks is the song's length for roll r in [0, SongSpan()]: its
// shortest length plus r ticks.
func SongTicks(r int) uint64 {
	return SongMin*clock.TicksPerDay/100 + uint64(max(r, 0))
}

// SongSpan is the spread (ticks) of the song's length.
func SongSpan() int { return (SongMax - SongMin) * clock.TicksPerDay / 100 }

// StartSong starts the song at tick now lasting ticks; false (nothing
// changes) when one is already playing or ticks is 0.
func (st *State) StartSong(now, ticks uint64) bool {
	if ticks == 0 || st.Song(now) {
		return false
	}
	st.SongUntil = now + ticks
	return true
}

// Song reports whether the sky song plays at tick now.
func (st State) Song(now uint64) bool { return now < st.SongUntil }

// SongView is the song's end tick while it plays at tick now (0: none),
// for WorldView.SkySongEnds.
func (st State) SongView(now uint64) uint64 {
	if st.Song(now) {
		return st.SongUntil
	}
	return 0
}
