package ebiten

import (
	"fmt"
	"image/color"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/domain/faction"
)

// facRow is one row of the Factions table (NG-1 §2.8). The adapter builds
// it from the generated factions; game.FactionViewsOf will take over once
// the application layer fills it.
type facRow struct {
	ID          int
	Name        string
	Type        faction.Type
	TypeLabel   string
	Leader      string // "Warden Iska Vell"
	Goodwill    int
	Relation    faction.Relation
	Settlements int
	Tech        string
	Emblem      string // ui_icon key
	Accent      color.NRGBA
	Capital     int // world tile of the capital, -1 none
	Tiles       []int
	Swarm       bool
}

func relationLabel(r faction.Relation) string {
	switch r {
	case faction.Hostile:
		return "Hostile"
	case faction.Friendly:
		return "Friendly"
	}
	return "Neutral"
}

func relationColor(r faction.Relation) color.RGBA {
	switch r {
	case faction.Hostile:
		return uiDanger
	case faction.Friendly:
		return uiGood
	}
	return uiTextDim
}

// factionRows lists fs as table rows, then the always-present Swarm.
func factionRows(fs []faction.Faction) []facRow {
	rows := make([]facRow, 0, len(fs)+1)
	for _, f := range fs {
		d := faction.DefOf(f.Type)
		r := facRow{
			ID: f.ID, Name: f.Name, Type: f.Type, TypeLabel: d.Label,
			Goodwill: f.Goodwill, Relation: f.Relation, Settlements: len(f.Settlements),
			Tech: techLabel(d.Tech), Emblem: "ui_icon/faction_" + d.Key,
			Accent: globeAccent(f.Type), Capital: -1,
		}
		if f.Leader.Name != "" {
			r.Leader = f.Leader.Title + " " + f.Leader.Name
		}
		for _, t := range f.Settlements {
			r.Tiles = append(r.Tiles, t)
		}
		if len(f.Settlements) > 0 {
			r.Capital = f.Settlements[0]
		}
		rows = append(rows, r)
	}
	sd := faction.DefOf(faction.Swarm)
	rows = append(rows, facRow{
		ID: -1, Name: "The Swarm", Type: faction.Swarm, TypeLabel: sd.Label,
		Leader: "Hive swarms: always hostile, no settlements", Goodwill: -100, Relation: faction.Hostile,
		Tech: "—", Emblem: "ui_icon/faction_swarm", Accent: globeAccent(faction.Swarm),
		Capital: -1, Swarm: true,
	})
	return rows
}

// goodwillText is "+12" / "-40".
func goodwillText(g int) string { return fmt.Sprintf("%+d", g) }

// hostileCapable reports slots include a human faction that can raid: one
// that starts hostile or can fall to hostile goodwill.
func hostileCapable(slots []faction.Slot) bool {
	for _, s := range slots {
		d := faction.DefOf(s.Type)
		if s.Count <= 0 || !s.Type.Human() {
			continue
		}
		if d.GoodwillMin <= -50 {
			return true
		}
	}
	return false
}

// slotIndex is the position of the type's slot, -1 if absent.
func slotIndex(slots []faction.Slot, t faction.Type) int {
	for i, s := range slots {
		if s.Type == t {
			return i
		}
	}
	return -1
}

// addableTypes are the configurable faction types still below their
// maximum count.
func addableTypes(slots []faction.Slot) []faction.Type {
	var out []faction.Type
	for _, d := range faction.All() {
		if d.Hidden || !d.Type.Human() || d.MaxCount <= 0 {
			continue
		}
		if i := slotIndex(slots, d.Type); i >= 0 && slots[i].Count >= d.MaxCount {
			continue
		}
		out = append(out, d.Type)
	}
	return out
}

// drawEmblem draws a faction glyph (white art) tinted with the faction
// accent, or a plain dot while the art is missing.
func (a *App) drawEmblem(dst *ebiten.Image, key string, tint color.NRGBA, x, y float32, size float64) {
	img := a.art.image(key, color.NRGBA{})
	if img == nil {
		vector.FillCircle(dst, x+float32(size)/2, y+float32(size)/2, float32(size)*0.32, tint, true)
		return
	}
	s := size / float64(img.Bounds().Dx())
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.ColorScale.ScaleWithColor(tint)
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(img, op)
}
