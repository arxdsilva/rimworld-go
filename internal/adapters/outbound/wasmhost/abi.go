// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package wasmhost

import (
	"encoding/binary"

	"rimworld-go/internal/application/scripting"
)

// Event layout of ABI v1 (doc.go "Guest ABI v1").
const (
	headerSize = 32
	recordSize = 32
	minBufSize = 64 // the smallest rw_buf_size a module may report
)

// eventTexts are the hook's two texts: on_incident's key and category,
// on_colonist_died's name.
func eventTexts(ev scripting.Event) (t1, t2 string) {
	switch ev.Hook {
	case scripting.OnIncident:
		return ev.Incident.Key, ev.Incident.Category
	case scripting.OnColonistDied:
		return ev.Name, ""
	}
	return "", ""
}

// appendEvent appends ev's header, the records of pawns and the texts to
// dst. For on_pawn the caller passes the one pawn.
func appendEvent(dst []byte, ev scripting.Event, pawns []scripting.Pawn) []byte {
	t1, t2 := eventTexts(ev)
	le := binary.LittleEndian
	dst = le.AppendUint32(dst, ABIVersion)
	dst = le.AppendUint32(dst, uint32(ev.Hook))
	dst = le.AppendUint64(dst, ev.Tick)
	dst = le.AppendUint32(dst, uint32(len(pawns))) // #nosec G115 -- eventSize <= bufSize (u32) at host_fire.go:88
	dst = le.AppendUint32(dst, uint32(len(t1)))    // #nosec G115 -- eventSize <= bufSize (u32) at host_fire.go:88
	dst = le.AppendUint32(dst, uint32(len(t2)))    // #nosec G115 -- eventSize <= bufSize (u32) at host_fire.go:88
	dst = le.AppendUint32(dst, 0)
	for _, p := range pawns {
		dst = appendRecord(dst, p)
	}
	dst = append(dst, t1...)
	return append(dst, t2...)
}

// eventSize is the encoded size of ev with n records.
func eventSize(ev scripting.Event, n int) int {
	t1, t2 := eventTexts(ev)
	return headerSize + n*recordSize + len(t1) + len(t2)
}

// appendRecord appends p's 32-byte record.
func appendRecord(dst []byte, p scripting.Pawn) []byte {
	le := binary.LittleEndian
	var flags uint32
	if p.Downed {
		flags |= 1
	}
	if p.Drafted {
		flags |= 2
	}
	dst = le.AppendUint64(dst, p.ID)
	dst = le.AppendUint32(dst, uint32(int32(p.X)))    // #nosec G115 -- i32 ABI bits; host map cell column (game/scripts.go:258)
	dst = le.AppendUint32(dst, uint32(int32(p.Y)))    // #nosec G115 -- i32 ABI bits; host map cell row (game/scripts.go:258)
	dst = le.AppendUint32(dst, uint32(int32(p.Food))) // #nosec G115 -- i32 ABI bits; host need bar 0-100 (needs.Percent, game/scripts.go:258)
	dst = le.AppendUint32(dst, uint32(int32(p.Rest))) // #nosec G115 -- i32 ABI bits; host need bar 0-100 (needs.Percent, game/scripts.go:258)
	dst = le.AppendUint32(dst, uint32(int32(p.Mood))) // #nosec G115 -- i32 ABI bits; host mood 0-100 (mood.Level or smoothed, scripts.go:258)
	return le.AppendUint32(dst, flags)
}
