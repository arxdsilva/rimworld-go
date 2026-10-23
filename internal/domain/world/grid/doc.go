// Package grid is the map's walkability: a width x height tile grid with
// one blocked bit per tile (a []uint64 bitset, so a pathfinding probe is
// a shift and a mask) and a version counter for reachability caches.
package grid
