package power

import (
	"cmp"
	"slices"
)

// HashSeed starts a Mix chain (FNV-1a's offset).
const HashSeed uint64 = 14695981039346656037

// Mix folds v into the FNV-1a hash h, low byte first.
func Mix(h, v uint64) uint64 {
	for range 8 {
		h ^= v & 0xff
		h *= 1099511628211
		v >>= 8
	}
	return h
}

// FlagBits packs flags into bits, the first at bit 0.
func FlagBits(flags ...bool) uint64 {
	bits := uint64(0)
	for i, f := range flags {
		if f {
			bits |= 1 << i
		}
	}
	return bits
}

// SortByNode sorts three parallel lists by node id.
func SortByNode[K any](ids []K, specs []Spec, nodes []Node) {
	idx := make([]int, len(nodes))
	for i := range idx {
		idx[i] = i
	}
	slices.SortFunc(idx, func(a, b int) int { return cmp.Compare(nodes[a].ID, nodes[b].ID) })
	oldIDs, oldSpecs, oldNodes := slices.Clone(ids), slices.Clone(specs), slices.Clone(nodes)
	for i, j := range idx {
		ids[i], specs[i], nodes[i] = oldIDs[j], oldSpecs[j], oldNodes[j]
	}
}
