// Package chunk divides a tile map into fixed 16×16 chunks, the unit of
// dirty tracking for regions and rooms (NG-1 §6.4). It is pure geometry
// plus a chunk bitset; it holds no game state.
package chunk
