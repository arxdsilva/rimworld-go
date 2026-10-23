// Package region partitions a tile map into regions, per 16×16 chunk
// (NG-1 §6.4): a region is a 4-connected flood of open cells within one
// chunk. Diagonal steps never cut corners in pathing.Find, so
// 4-connectivity is exactly A* connectivity.
//
// Map is the walkability partition: doors, fences, gates and traps are
// 1-cell regions carrying a flag, and each movement layer (colonist,
// hostile, sapper, tame, wild) gets connected-component labels over the
// region graph, so "can X reach Y" is a label comparison. Rooms is the
// room partition (boundaries: rock, walls, doors): rooms are components
// of room-regions, numbered exactly as room.Detect numbers them.
//
// Both are updated incrementally: a changed cell marks its chunk dirty,
// and dirty chunks are re-flooded and relinked in index order before the
// next query. Everything is deterministic and holds no game state of its
// own beyond what callers feed in.
package region
