package render

import "image"

// drawBridgesAndVaults paints MAPGEN-3's features over a view: the deck
// of every old road bridge, and each sealed vault's floor, wall ring,
// loot, 1x2 caskets and dormant defender spots (later layers win).
func (v *view) drawBridgesAndVaults(img *image.RGBA) {
	for _, b := range v.res.Bridges {
		v.set(img, b.X, b.Y, BridgeColor)
	}
	for _, vl := range v.res.Vaults {
		for _, f := range vl.Floors {
			v.set(img, f.X, f.Y, VaultFloorColor)
		}
		for _, w := range vl.Walls {
			v.set(img, w.X, w.Y, VaultWallColor)
		}
		for _, l := range vl.Loot {
			v.set(img, l.X, l.Y, LootColor)
		}
		for _, c := range vl.Caskets {
			v.set(img, c.X, c.Y, CasketColor)
			v.set(img, c.X, c.Y+1, CasketColor)
		}
		for _, d := range vl.DefenderSpots {
			v.set(img, d.X, d.Y, DefenderColor)
		}
	}
}
