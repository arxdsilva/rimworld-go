package quest

import (
	"strconv"

	"rimworld-go/internal/domain/economy/item"
)

// ArgKind says what a detail line's argument is, so the UI can show it:
// a faction's name, a date or the time left, a tile's name, an item's
// label (QST-125). The quest never formats text itself.
type ArgKind uint8

const (
	// ArgNum is a plain count or amount (N).
	ArgNum ArgKind = iota
	// ArgText is shown as is (S: a pawn's name).
	ArgText
	// ArgFaction is a faction ref, 1 + its index (N; 0 none).
	ArgFaction
	// ArgTick is an absolute simulation tick (N): a date or the time left.
	ArgTick
	// ArgTile is a world tile (N).
	ArgTile
	// ArgItem is an item.Kind (N).
	ArgItem
)

// Arg is one argument of a detail line.
type Arg struct {
	Kind ArgKind
	N    int64
	S    string
}

// Num, Text, Faction, Tick, Tile and Item build one argument of their kind.
func Num(n int) Arg        { return Arg{Kind: ArgNum, N: int64(n)} }
func Text(s string) Arg    { return Arg{Kind: ArgText, S: s} }
func Faction(ref int) Arg  { return Arg{Kind: ArgFaction, N: int64(ref)} }
func Tick(t uint64) Arg    { return Arg{Kind: ArgTick, N: int64(t)} }
func Tile(t int) Arg       { return Arg{Kind: ArgTile, N: int64(t)} }
func Item(k item.Kind) Arg { return Arg{Kind: ArgItem, N: int64(k)} }
func (a Arg) String() string { // the raw value (tests, logs); the UI formats by Kind
	if a.Kind == ArgText {
		return a.S
	}
	return strconv.FormatInt(a.N, 10)
}
