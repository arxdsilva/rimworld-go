package windows

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"rimworld-go/internal/adapters/inbound/ebiten/art"
	"rimworld-go/internal/adapters/inbound/ebiten/gfx"
	"rimworld-go/internal/adapters/inbound/ebiten/globe"
	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
	"rimworld-go/internal/domain/world/planet"
)

// The World tab's travel layer (AX-9, MAIN_UI_SCREENS §33/§35 travel
// slice): pods in flight and mineral sites marked on the globe and listed
// under the caravans, and the transport pods' launch target picker
// (opened from a pod's Launch button): click a tile, read its distance
// against the pods' range, then land, gift or visit there (LaunchPods).

const (
	podFlightIcon  = "ui_icon/pod_flight"
	miningSiteIcon = "ui_icon/mining_site"
	travelRowH     = 22
	// Launch picker button ids (journeys press them, inside the World
	// tab's modal scope).
	LaunchDropID   = "world.launch.drop"
	LaunchGiftID   = "world.launch.gift"
	LaunchVisitID  = "world.launch.visit"
	LaunchCancelID = "world.launch.cancel"
	launchBarH     = 64
)

// LaunchFor opens the World tab picking the launch target of pod's group.
func (w World) LaunchFor(pod entity.EntityID) {
	if w.KitID() == WorldModalID {
		w.CloseQuietly()
	}
	w.launchWith(&worldUI{picked: -1, launch: pod})
}

// travelTiles are the tiles the travel layer highlights: pods' targets and
// mining sites.
func travelTiles(v game.WorldView) []int {
	var out []int
	for _, f := range v.AX.Pods {
		out = append(out, int(f.To))
	}
	for _, s := range v.AX.Sites {
		out = append(out, int(s.Tile))
	}
	return out
}

// travelMark is one marker on the globe: a unit-sphere point and its icon.
type travelMark struct {
	pt   [3]float32
	icon string
}

// travelMarks are the pods in flight (part way along their arc, by the
// time left; the distance is the planet's hop count) and the mining sites.
func travelMarks(p *planet.Planet, v game.WorldView, now uint64) []travelMark {
	var out []travelMark
	n := p.TileCount()
	for _, f := range v.AX.Pods {
		if int(f.From) < 0 || int(f.From) >= n || int(f.To) < 0 || int(f.To) >= n {
			continue
		}
		total := float64(max(1, p.Hops(int(f.From), int(f.To), caravan.PodMaxRange+1)) * caravan.PodTicksPerTile)
		t := 1.0
		if f.Arrive > now {
			t = math.Max(0, 1-float64(f.Arrive-now)/total)
		}
		out = append(out, travelMark{pt: slerpish(p.Center[f.From], p.Center[f.To], t), icon: podFlightIcon})
	}
	for _, s := range v.AX.Sites {
		if int(s.Tile) >= 0 && int(s.Tile) < n {
			out = append(out, travelMark{pt: p.Center[s.Tile], icon: miningSiteIcon})
		}
	}
	return out
}

// slerpish is the point t of the way from a to b, back on the unit sphere.
func slerpish(a, b [3]float32, t float64) [3]float32 {
	var o [3]float32
	l := 0.0
	for i := range o {
		v := float64(a[i]) + (float64(b[i])-float64(a[i]))*t
		o[i] = float32(v)
		l += v * v
	}
	if l = math.Sqrt(l); l > 0 {
		for i := range o {
			o[i] = float32(float64(o[i]) / l)
		}
	}
	return o
}

// drawTravelMarks draws the travel markers on the globe's facing side, in
// the caravan badge style.
func drawTravelMarks(dst *ebiten.Image, g *globe.View, c *art.Catalog, marks []travelMark) {
	size := globe.Clamp(g.TilePx()*2, 14, 40)
	for i := range marks {
		sx, sy, vz := g.WorldPt(&marks[i].pt)
		if vz < g.Zh+0.04 {
			continue
		}
		vector.FillCircle(dst, sx, sy, float32(size*0.56), color.RGBA{R: uikit.Space.R, G: uikit.Space.G, B: uikit.Space.B, A: 170}, true)
		var img *ebiten.Image
		if c != nil {
			img = c.Image(marks[i].icon, color.NRGBA{})
		}
		if img == nil {
			vector.FillCircle(dst, sx, sy, float32(size*0.3), uikit.Heading, true)
			continue
		}
		b := img.Bounds()
		s := size / float64(b.Dx())
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(float64(sx)-size/2, float64(sy)-float64(b.Dy())*s/2)
		dst.DrawImage(img, op)
	}
}

// travelList declares the pods in flight and mining sites at the bottom of
// the list column and returns the rest of it for the caravans.
func (w World) travelList(r gfx.Rect) gfx.Rect {
	v := w.sh.World()
	n := len(v.AX.Pods) + len(v.AX.Sites)
	if n == 0 {
		return r
	}
	h := min(r.H/2, float32(28+(n+1)*travelRowH))
	top, bot := r.SplitV(r.H - h)
	now := uint64(w.sh.Tick())
	y := bot.Y
	line := func(s string, st gfx.TextStyle) {
		if y+travelRowH <= bot.Bottom() {
			w.kit().Label(bot.X, y, gfx.Ellipsize(s, st, float64(bot.W-8)), st)
		}
		y += travelRowH
	}
	hs, sm := gfx.WithColor(gfx.Label, uikit.Heading), gfx.WithColor(gfx.Small, uikit.TextDim)
	if len(v.AX.Pods) > 0 {
		line("Pods in flight", hs)
		for _, f := range v.AX.Pods {
			line(fmt.Sprintf("%d pods to tile %d · lands in %s", f.Count, f.To, hoursLeft(f.Arrive, now)), sm)
		}
	}
	if len(v.AX.Sites) > 0 {
		line("Mineral sites", hs)
		for _, s := range v.AX.Sites {
			line(fmt.Sprintf("%s · tile %d · gone in %s", capFirst(s.Mineral), s.Tile, hoursLeft(s.Expires, now)), sm)
		}
	}
	top.H -= 8
	return top
}

// hoursLeft is the time from now to at ("3 h", "2 d").
func hoursLeft(at, now uint64) string {
	if at <= now {
		return "moments"
	}
	h := int((at - now) / clock.TicksPerHour)
	if h >= 48 {
		return fmt.Sprintf("%d d", h/24)
	}
	return fmt.Sprintf("%d h", max(1, h))
}

func capFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// LaunchTarget is the picker's verdict on tile t for pods of range rng
// launched from home: its name, distance, and why each mode is refused
// ("" allowed).
type LaunchTarget struct {
	Name              string
	Dist              int
	Drop, Gift, Visit string
}

// launchTarget judges tile t (sites are the settlements).
func launchTarget(p *planet.Planet, home, t, rng int, sites []game.WorldSiteView) LaunchTarget {
	lt := LaunchTarget{Name: fmt.Sprintf("tile %d", t)}
	if t < 0 || t >= p.TileCount() {
		why := "Click a tile on the globe first."
		lt.Drop, lt.Gift, lt.Visit = why, why, why
		return lt
	}
	if t == home {
		lt.Name = "the colony"
	}
	lt.Dist = p.Hops(home, t, caravan.PodMaxRange+1)
	var site *game.WorldSiteView
	for i := range sites {
		if sites[i].Tile == t {
			site, lt.Name = &sites[i], sites[i].Name
		}
	}
	lt.Gift, lt.Visit = "A gift needs a settlement.", "Only a friendly settlement takes visitors."
	if site != nil {
		lt.Gift = ""
		if site.Visitable {
			lt.Visit = ""
		}
		lt.Drop = "Land on a settlement as a visit (or a gift)."
	}
	if lt.Dist > rng {
		why := fmt.Sprintf("Out of range: %d tiles, the fuel reaches %d.", lt.Dist, rng)
		lt.Drop, lt.Gift, lt.Visit = why, why, why
	}
	return lt
}

// launchBar declares the picker at the bottom of the globe area r.
func (w World) launchBar(st *worldUI, r gfx.Rect, p *planet.Planet) {
	var pod game.BuildingView
	for _, b := range w.sh.World().Buildings {
		if b.ID == st.launch {
			pod = b
		}
	}
	var sites []game.WorldSiteView
	if src, ok := w.sh.Env().Game.(WorldSource); ok {
		sites = src.WorldSites()
	}
	home := w.sh.Env().Game.WorldMap().Tile
	lt := launchTarget(p, home, st.picked, pod.AX.Range, sites)
	bar := gfx.Rect{X: r.X, Y: r.Bottom() - launchBarH, W: r.W, H: launchBarH}
	w.kit().Queue(func(dst *ebiten.Image) {
		uikit.FillRect(dst, bar, color.RGBA{R: uikit.Space.R, G: uikit.Space.G, B: uikit.Space.B, A: 220})
	})
	head := fmt.Sprintf("Launch target: click a tile · pods reach %d tiles, %d kg aboard", pod.AX.Range, pod.AX.LoadKg)
	if st.picked >= 0 {
		head = fmt.Sprintf("Launch target: %s · %d tiles of %d", lt.Name, lt.Dist, pod.AX.Range)
	}
	w.kit().Label(bar.X+8, bar.Y+4, gfx.Ellipsize(head, gfx.Body, float64(bar.W-16)), gfx.Body)
	bw, by := float32(120), bar.Y+30
	send := func(id, label, why, tip string, mode uint8, x float32) {
		if w.kit().Button(id, gfx.Rect{X: x, Y: by, W: bw, H: 26}, label, uikit.BtnOpts{Disabled: why != "", Reason: why, Tip: tip}) {
			w.sh.Enqueue(command.Command{Kind: command.LaunchPods, Target: uint64(st.launch), Count: st.picked, Mode: mode})
			w.CloseKit()
		}
	}
	send(LaunchDropID, "Land here", lt.Drop, "Land here\nOn the colony the pods come down at its drop spot; anywhere else the passengers wait as a caravan.", command.PodArriveDrop, bar.X+8)
	send(LaunchGiftID, "Send as gift", lt.Gift, "Send as gift\nThe settlement keeps the goods and animals; goodwill rises with their value.", command.PodArriveGift, bar.X+16+bw)
	send(LaunchVisitID, "Visit", lt.Visit, "Visit\nThe passengers stay a day as guests, then wait for orders.", command.PodArriveVisit, bar.X+24+2*bw)
	if w.kit().Button(LaunchCancelID, gfx.Rect{X: bar.Right() - bw - 8, Y: by, W: bw, H: 26}, "Cancel", uikit.BtnOpts{}) {
		st.launch = 0
	}
}

// launchList declares, while picking a launch target, the settlements and
// mining sites in the pods' range as quick picks (the globe works too).
func (w World) launchList(st *worldUI, r gfx.Rect) {
	head, rest := r.SplitV(28)
	w.kit().Label(head.X, head.Y, "Targets in range", gfx.WithColor(gfx.Label, uikit.Heading))
	var pod game.BuildingView
	for _, b := range w.sh.World().Buildings {
		if b.ID == st.launch {
			pod = b
		}
	}
	p := st.planet
	if p == nil {
		return
	}
	home := w.sh.Env().Game.WorldMap().Tile
	type pick struct {
		tile  int
		label string
	}
	picks := []pick{{home, "The colony (land at home)"}}
	if src, ok := w.sh.Env().Game.(WorldSource); ok {
		for _, s := range src.WorldSites() {
			if d := p.Hops(home, s.Tile, caravan.PodMaxRange+1); d <= pod.AX.Range {
				picks = append(picks, pick{s.Tile, fmt.Sprintf("%s (%s, %d tiles)", s.Name, s.Relation, d)})
			}
		}
	}
	for _, s := range w.sh.World().AX.Sites {
		if d := p.Hops(home, int(s.Tile), caravan.PodMaxRange+1); d <= pod.AX.Range {
			picks = append(picks, pick{int(s.Tile), fmt.Sprintf("%s site (%d tiles)", capFirst(s.Mineral), d)})
		}
	}
	area := w.kit().ScrollArea("world.launch.targets", rest, float32(len(picks)*worldRowH/2), uikit.ScrollOpts{Step: worldRowH})
	y := area.View.Y - area.Off
	for _, pk := range picks {
		row := gfx.Rect{X: area.View.X, Y: y, W: area.View.W - 4, H: worldRowH/2 - 6}
		y += worldRowH / 2
		if row.Y < area.View.Y || row.Bottom() > area.View.Bottom() {
			continue
		}
		if w.kit().Button(fmt.Sprintf("world.launch.target.%d", pk.tile), row, gfx.Ellipsize(pk.label, gfx.Body, float64(row.W-12)),
			uikit.BtnOpts{Selected: st.picked == pk.tile}) {
			st.picked = pk.tile
			if st.globe != nil {
				st.globe.Center(pk.tile)
			}
		}
	}
}
