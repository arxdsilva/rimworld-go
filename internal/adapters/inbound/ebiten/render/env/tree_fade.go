package env

import (
	"math"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
)

// Tree parts and the see-through canopy (PLAY-010 2.9). Every tree key
// also exists as <key>_trunk and <key>_canopy on the same canvas and
// origin. The trunk (and the shadow) never fade; the canopy eases to
// TreeSeeThrough while a creature, the hovered thing or the Architect
// ghost stands on the tree's tile, the tile above or the two diagonals
// above, so whoever walks behind a tree stays visible.

const (
	TreeTrunkSuffix  = "_trunk"
	TreeCanopySuffix = "_canopy"
	TreeSeeThrough   = 0.4
	treeFadeSecs     = 0.2
	treeFadeFPS      = 60.0 // present frames per second
)

// treeFadeState holds a tree's canopy alpha spring while it differs from
// opaque. Settled opaque trees hold no entry; entries the view stops
// drawing are dropped.
type treeFadeState struct {
	Alpha float64
	at    uint64 // present frame of the last update
}

// TreeFx is the see-through canopy state.
type TreeFx struct {
	Fades map[uint32]*treeFadeState
	focus [][2]int // hovered entity or Architect ghost tiles (SetTreeFocus)
	swept uint64
}

// SetTreeFocus lists the tiles of the hovered/selected entity or the
// Architect ghost; canopies over them turn see-through too. Replaces
// the previous list; call with nil to clear.
func (fx *TreeFx) SetTreeFocus(cells [][2]int) {
	fx.focus = append(fx.focus[:0], cells...)
}

// treeCanopyTarget is the alpha the canopy of the tree at (x, y) eases to;
// cr marks the tiles creatures stand on this frame.
func (fx *TreeFx) treeCanopyTarget(x, y int, cr map[[2]int]bool) float64 {
	cells := [4][2]int{{x, y}, {x, y - 1}, {x - 1, y - 1}, {x + 1, y - 1}}
	for _, c := range cells {
		if cr[c] {
			return TreeSeeThrough
		}
	}
	for _, f := range fx.focus {
		for _, c := range cells {
			if f == c {
				return TreeSeeThrough
			}
		}
	}
	return 1
}

// TreeCanopyAlpha advances and returns the canopy alpha of the tree at
// (x, y), given the tiles creatures stand on (creatures). Reduced Motion
// snaps to the target.
func (fx *TreeFx) TreeCanopyAlpha(v *render.View, x, y int, creatures map[[2]int]bool) float32 {
	target := fx.treeCanopyTarget(x, y, creatures)
	k := TileKey(x, y)
	st := fx.Fades[k]
	if st == nil {
		if target == 1 {
			return 1
		}
		if fx.Fades == nil {
			fx.Fades = map[uint32]*treeFadeState{}
		}
		st = &treeFadeState{Alpha: 1, at: v.Present}
		if v.ReducedMotion {
			st.Alpha = target
		}
		fx.Fades[k] = st
	}
	if v.ReducedMotion {
		st.Alpha = target
	} else if dt := v.Present - st.at; dt > 0 {
		step := math.Min(float64(dt), 12) / (treeFadeSecs * treeFadeFPS)
		st.Alpha += (target - st.Alpha) * (1 - math.Exp(-3*step)) // ~95% in treeFadeSecs
		if math.Abs(target-st.Alpha) < 0.01 {
			st.Alpha = target
		}
	}
	st.at = v.Present
	if st.Alpha == 1 && target == 1 {
		delete(fx.Fades, k)
		return 1
	}
	return float32(st.Alpha)
}

// SweepTreeFades drops springs of trees no longer drawn (they left the
// view): once per frame, from drawArt.
func (fx *TreeFx) SweepTreeFades(v *render.View) {
	if len(fx.Fades) == 0 || fx.swept == v.Present {
		return
	}
	fx.swept = v.Present
	for k, st := range fx.Fades {
		if v.Present-st.at > 2 {
			delete(fx.Fades, k)
		}
	}
}

// TreePartKeys returns the trunk and canopy keys of a tree sprite, or
// ok=false when either part is missing (the full sprite is drawn).
func TreePartKeys(v *render.View, key string) (trunk, canopy string, ok bool) {
	trunk, canopy = key+TreeTrunkSuffix, key+TreeCanopySuffix
	return trunk, canopy, v.Art.Has(trunk) && v.Art.Has(canopy)
}

// TreePartCoverageKeys lists the whole-tree keys that must have parts.
func TreePartCoverageKeys() []string {
	var out []string
	for _, sp := range []string{"oak", "birch"} {
		for v := 0; v < 3; v++ {
			base := "plant/tree_" + sp + "_0" + string(rune('0'+v))
			out = append(out, base, base+"_autumn", base+"_bare")
		}
	}
	for v := 0; v < 3; v++ {
		n := "0" + string(rune('0'+v))
		out = append(out, "plant/tree_jungle_"+n,
			"plant/tree_pine_"+n, "plant/tree_pine_"+n+"_snow")
	}
	return out
}
