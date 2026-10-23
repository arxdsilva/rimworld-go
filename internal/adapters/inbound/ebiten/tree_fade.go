package ebiten

import "math"

// Tree parts and the see-through canopy (PLAY-010 2.9). Every tree key
// also exists as <key>_trunk and <key>_canopy on the same canvas and
// origin. The trunk (and the shadow) never fade; the canopy eases to
// treeSeeThrough while a creature, the hovered thing or the Architect
// ghost stands on the tree's tile, the tile above or the two diagonals
// above, so whoever walks behind a tree stays visible.

const (
	treeTrunkSuffix  = "_trunk"
	treeCanopySuffix = "_canopy"
	treeSeeThrough   = 0.4
	treeFadeSecs     = 0.2
	treeFadeFPS      = 60.0 // present frames per second
)

// treeFadeState holds a tree's canopy alpha spring while it differs from
// opaque. Settled opaque trees hold no entry; entries the view stops
// drawing are dropped.
type treeFadeState struct {
	alpha float64
	at    uint64 // present frame of the last update
}

// treeFx is the renderer's see-through canopy state.
type treeFx struct {
	fades map[uint32]*treeFadeState
	focus [][2]int // hovered entity or Architect ghost tiles (SetTreeFocus)
	swept uint64
}

// SetTreeFocus lists the tiles of the hovered/selected entity or the
// Architect ghost; canopies over them turn see-through too. Replaces
// the previous list; call with nil to clear.
func (r *Renderer) SetTreeFocus(cells [][2]int) {
	r.treeFx.focus = append(r.treeFx.focus[:0], cells...)
}

// treeCanopyTarget is the alpha the canopy of the tree at (x, y) eases to.
func (r *Renderer) treeCanopyTarget(x, y int) float64 {
	cells := [4][2]int{{x, y}, {x, y - 1}, {x - 1, y - 1}, {x + 1, y - 1}}
	cr := r.lod.creatures
	for _, c := range cells {
		if cr[c] {
			return treeSeeThrough
		}
	}
	for _, f := range r.treeFx.focus {
		for _, c := range cells {
			if f == c {
				return treeSeeThrough
			}
		}
	}
	return 1
}

// treeCanopyAlpha advances and returns the canopy alpha of the tree at
// (x, y). Reduced Motion snaps to the target.
func (r *Renderer) treeCanopyAlpha(x, y int) float32 {
	target := r.treeCanopyTarget(x, y)
	fx := &r.treeFx
	k := envTileKey(x, y)
	st := fx.fades[k]
	if st == nil {
		if target == 1 {
			return 1
		}
		if fx.fades == nil {
			fx.fades = map[uint32]*treeFadeState{}
		}
		st = &treeFadeState{alpha: 1, at: r.present}
		if r.reducedMotion {
			st.alpha = target
		}
		fx.fades[k] = st
	}
	if r.reducedMotion {
		st.alpha = target
	} else if dt := r.present - st.at; dt > 0 {
		step := math.Min(float64(dt), 12) / (treeFadeSecs * treeFadeFPS)
		st.alpha += (target - st.alpha) * (1 - math.Exp(-3*step)) // ~95% in treeFadeSecs
		if math.Abs(target-st.alpha) < 0.01 {
			st.alpha = target
		}
	}
	st.at = r.present
	if st.alpha == 1 && target == 1 {
		delete(fx.fades, k)
		return 1
	}
	return float32(st.alpha)
}

// sweepTreeFades drops springs of trees no longer drawn (they left the
// view): once per frame, from drawArt.
func (r *Renderer) sweepTreeFades() {
	fx := &r.treeFx
	if len(fx.fades) == 0 || fx.swept == r.present {
		return
	}
	fx.swept = r.present
	for k, st := range fx.fades {
		if r.present-st.at > 2 {
			delete(fx.fades, k)
		}
	}
}

// treePartKeys returns the trunk and canopy keys of a tree sprite, or
// ok=false when either part is missing (the full sprite is drawn).
func (r *Renderer) treePartKeys(key string) (trunk, canopy string, ok bool) {
	trunk, canopy = key+treeTrunkSuffix, key+treeCanopySuffix
	return trunk, canopy, r.art.has(trunk) && r.art.has(canopy)
}

// treePartCoverageKeys lists the whole-tree keys that must have parts.
func treePartCoverageKeys() []string {
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
