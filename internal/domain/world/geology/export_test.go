package geology

// Internals exposed to the external geology_test package, which needs
// generation (and generation → plant → item imports geology).
var (
	Depths     = depths
	DeepCells  = deepCells
	PickStones = pickStones
)

// VeinMax is the largest vein of ore o.
func VeinMax(o Ore) int {
	for _, v := range veinSpecs {
		if v.ore == o {
			return v.minSize + v.sizeSpan - 1
		}
	}
	return 0
}

// VeinCount is how many veins of o a map with deep depth-≥2 cells gets.
func VeinCount(deep int, o Ore) int {
	for _, v := range veinSpecs {
		if v.ore == o {
			return v.count(deep)
		}
	}
	return 0
}
