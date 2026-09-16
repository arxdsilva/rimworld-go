// Package plant is harvestable wild plants (no grow-zone).
package plant

const (
	HarvestWork  = 30
	HarvestYield = 5
)

// Wild is a harvestable map plant.
type Wild struct {
	X, Y        int
	Harvestable bool
	Designated  bool
}
