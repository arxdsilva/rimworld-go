// Package worldinfo is the world-map reading aids shared by the Select
// Landing Site screen and the in-game world view (WG-3): the tile card
// (BuildTileInfo), the feature SearchBox, the random-site FilterPanel and
// the Legend. It reads a finished *planet.Planet and a units.Temp; it
// never changes either.
package worldinfo

import (
	"fmt"
	"image/color"
	"slices"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/uikit"
	"rimworld-go/internal/adapters/inbound/ebiten/units"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/planet"
	"rimworld-go/internal/domain/world/site"
)

// Row is a label/value line of the tile card; Tip is its hover card and
// Col the value colour.
type Row struct {
	Label, Value, Tip string
	Col               color.RGBA
}

// Quadrum is one column of the tile card's temperature table: the season
// the quadrum falls in at this tile and its mean temperature, formatted.
type Quadrum struct {
	Season string
	Temp   string
}

// TileInfo is everything the card shows for one tile.
type TileInfo struct {
	Tile       int
	Biome      biome.ID
	Name       string
	Blurb      string
	Difficulty string
	BiomeTip   string
	Rows       []Row
	Quadrums   []Quadrum // empty for water
	Warnings   []string
	Invalid    string // why the tile can't be landed on ("" valid)
	Outside    bool
}

// Opts tune a card: SizeNote is the map-size memory warning ("" none),
// Units formats temperatures, Day is the day of the year the movement
// difficulty row reads (the start of the colony).
type Opts struct {
	SizeNote string
	Units    units.Temp
	Day      int
	// Live, when set, says a planet settlement still stands (the running
	// game's razed bases stay on the view's planet copy; MAP-05).
	Live func(tile int) bool
}

// hoursPerTileDay is the days a caravan takes to cross a difficulty-1
// tile (wiki: Caravan).
const hoursPerTileDay = 0.06

// CoordText writes a position as "34.2°S 121.7°W" from tenths of a degree.
func CoordText(lat, lon int16) string {
	ns, ew := "N", "E"
	if lat < 0 {
		ns, lat = "S", -lat
	}
	if lon < 0 {
		ew, lon = "W", -lon
	}
	return fmt.Sprintf("%d.%d°%s %d.%d°%s", lat/10, lat%10, ns, lon/10, lon%10, ew)
}

// TimeZoneText is "UTC+3", "UTC-5" or "UTC+0".
func TimeZoneText(z int) string {
	if z < 0 {
		return fmt.Sprintf("UTC-%d", -z)
	}
	return fmt.Sprintf("UTC+%d", z)
}

// HillsTip explains hilliness for the local map.
func HillsTip(h site.Hills) string {
	switch h {
	case site.SmallHills:
		return "Small hills\nA few rocky rises. Some mining, easy building."
	case site.LargeHills:
		return "Large hills\nMuch of the map is rock. Good for mining and defence."
	case site.Mountainous:
		return "Mountainous\nMostly mountain. Building room is scarce but ore is plentiful."
	}
	return "Flat\nOpen ground. Easy to build, little rock to mine."
}

// ForageText is the share of wild food found, "75%".
func ForageText(b biome.ID) string { return fmt.Sprintf("%d%%", biome.DefOf(b).Forage) }

// DiseaseText is "every ~50 days".
func DiseaseText(b biome.ID) string {
	return fmt.Sprintf("every ~%d days", biome.DefOf(b).DiseaseMTB)
}

// WildlifeLevel is the animal density label of a biome.
func WildlifeLevel(b biome.ID) string {
	w := biome.DefOf(b).Wildlife
	switch {
	case w <= 15:
		return "Sparse"
	case w <= 45:
		return "Low"
	case w <= 85:
		return "Moderate"
	case w < 100:
		return "Plentiful"
	}
	return "Abundant"
}

// TravelHours is the hours a caravan takes to cross a tile of movement
// difficulty d (0: impassable, reported as 0).
func TravelHours(d float64) float64 { return hoursPerTileDay * 24 * d }

// moveText is a movement difficulty as "x2.5" ("Impassable" for water).
func moveText(d float64) string {
	if d <= 0 {
		return "Impassable"
	}
	return fmt.Sprintf("x%.1f", d)
}

func hoursText(d float64) string {
	if d <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f h", TravelHours(d))
}

// roadText names the road classes that meet tile i.
func roadText(p *planet.Planet, i int, hasRoad bool) string {
	var seen [planet.NumRoadClasses]bool
	for k, e := range p.Roads {
		if int(e[0]) == i || int(e[1]) == i {
			seen[p.RoadClassOf(k)] = true
		}
	}
	var labels []string
	for c := planet.RoadClass(0); c < planet.NumRoadClasses; c++ {
		if seen[c] {
			labels = append(labels, c.Label())
		}
	}
	switch {
	case len(labels) > 0:
		return strings.Join(labels, ", ")
	case hasRoad:
		return planet.DirtRoad.Label()
	}
	return "None"
}

// BuildTileInfo reads tile i of p for the card in Celsius at day 0;
// sizeNote is the memory warning for the chosen map size ("" none).
func BuildTileInfo(p *planet.Planet, i int, sizeNote string) TileInfo {
	return BuildTileInfoWith(p, i, Opts{SizeNote: sizeNote})
}

// BuildTileInfoWith reads tile i of p for the card.
func BuildTileInfoWith(p *planet.Planet, i int, o Opts) TileInfo {
	if i < 0 || i >= p.TileCount() {
		return TileInfo{Tile: i, Outside: true}
	}
	u := o.Units
	s := p.Site(i)
	d := biome.DefOf(p.Biome[i])
	ti := TileInfo{Tile: i, Biome: p.Biome[i], Name: d.Name, Blurb: d.Blurb, Difficulty: d.Difficulty}
	if p.Water[i] == planet.Ocean {
		ti.Name, ti.Blurb, ti.Difficulty = "Ocean", "Open sea. Nobody can land here.", ""
	} else if p.Water[i] == planet.Lake {
		ti.Name, ti.Blurb, ti.Difficulty = "Lake", "Fresh water. Nobody can land here.", ""
	}
	ti.BiomeTip = d.Name + "\n" + strings.Join(LandingDetail(game.BiomeInfoOf(p.Biome[i]), 60, u)[2:], "\n")
	if ok, reason := p.ValidLanding(i); !ok {
		ti.Invalid = reason
	}
	lo, hi := s.Climate.Range()
	grow := p.GrowingDays(i)
	growText := fmt.Sprintf("%d of %d days", grow, clock.DaysPerYear)
	if grow >= clock.DaysPerYear {
		growText = "Year-round"
	}
	var stones []string
	for _, st := range s.Stones {
		stones = append(stones, st.Label())
	}
	water := []string{}
	if l := s.River.Size.Label(); l != "" {
		water = append(water, l)
	}
	if s.Coast != 0 {
		water = append(water, "Coast")
	}
	if s.Lakes > 0 {
		water = append(water, "Lake")
	}
	waterText, waterTip := "None", "Water\nRivers, coast and lakes shape the map and give water to build beside."
	if len(water) > 0 {
		waterText = strings.Join(water, ", ")
	}
	if s.River.Size == site.HugeRiver {
		waterTip += "\nA huge river is too wide to wade; the map is split by it."
	}
	peak := s.Climate.PeakDay
	mv := p.MoveDifficulty(i, o.Day)
	moveTip := fmt.Sprintf("Movement difficulty\nHow slow it is for a caravan to cross this tile. Today x%.1f, summer x%.1f, winter x%.1f. Roads halve it.",
		mv, p.MoveDifficulty(i, peak), p.MoveDifficulty(i, peak+2*clock.DaysPerQuadrum))
	var travel []string
	for q := 0; q < clock.QuadrumsPerYear; q++ {
		day := q*clock.DaysPerQuadrum + clock.DaysPerQuadrum/2
		travel = append(travel, fmt.Sprintf("%s %s", quadrumSeason(s, day), hoursText(p.MoveDifficulty(i, day))))
	}
	zone := p.TimeZone(i)
	ti.Rows = []Row{
		{"Terrain", s.Hills.Label(), HillsTip(s.Hills), uikit.Text},
		{"Coordinates", CoordText(p.Lat[i], p.Lon[i]), "", uikit.Text},
		{"Time zone", TimeZoneText(zone), "Time zone\nHours from the prime meridian (one zone per 15 degrees of longitude).", uikit.Text},
		{"Elevation", fmt.Sprintf("%d m", p.Elev[i]), "", uikit.Text},
		{"Avg temperature", u.Degrees(float64(p.Temp[i])/10, 1), "Average temperature\nThe yearly mean at this tile.", uikit.Text},
		{"Summer / winter", u.Whole(float64(hi)/10) + " / " + u.Whole(float64(lo)/10), "The warmest afternoon and the coldest night of the year.", uikit.Text},
		{"Rainfall", fmt.Sprintf("%d mm/year", p.Rain[i]), "", uikit.Text},
		{"Growing period", growText, "Growing period\nDays a year warm enough for crops to grow (daily mean above " + u.Whole(3) + ").", uikit.Text},
		{"Stone types", strings.Join(stones, ", "), "Stone types\nWhat the local rock is made of. It sets the building stone and ore you can mine.", uikit.Text},
		{"Water", waterText, waterTip, uikit.Text},
		{"Caves", yesNo(s.Caves), "Caves\nCave systems under the hills hide ore, and sometimes things worse.", uikit.Text},
		{"Roads", roadText(p, i, s.Road != 0), "Roads\nAncient and dirt roads halve a caravan's movement difficulty.", uikit.Text},
	}
	ti.Rows = append(ti.Rows, baseRows(p, i, o.Live)...)
	ti.Rows = append(ti.Rows,
		Row{"Movement difficulty", moveText(mv), moveTip, uikit.Text},
		Row{"Caravan per tile", strings.Join(travel, ", "), "Caravan travel\nHours a caravan spends crossing this tile in each season (" +
			fmt.Sprintf("%.2f", hoursPerTileDay) + " days times the movement difficulty).", uikit.Text},
		Row{"Forage", ForageText(p.Biome[i]), "Forage\nHow much wild food foraging finds here, as a share of the best biome.", uikit.Text},
		Row{"Wildlife", WildlifeLevel(p.Biome[i]), "Wildlife\nHow many wild animals live here.", uikit.Text},
		Row{"Disease", DiseaseText(p.Biome[i]), "Disease\nThe average time between outbreaks that sicken a colonist here.", uikit.Text},
	)
	if p.Water[i] != planet.Land {
		ti.Rows = ti.Rows[:3]
	} else {
		ti.Quadrums = quadrums(p, i, s, u)
	}
	switch p.Biome[i] {
	case biome.IceSheet, biome.SeaIce, biome.ExtremeDesert:
		ti.Warnings = append(ti.Warnings, "Extreme climate")
	}
	if p.Water[i] == planet.Land && grow < 15 {
		ti.Warnings = append(ti.Warnings, "Very short growing season")
	}
	if b, ok := p.NearBase(i); ok && p.Water[i] == planet.Land {
		ti.Warnings = append(ti.Warnings, fmt.Sprintf("Near a faction base: %s, %d tiles away", baseName(p, b), b.Hops))
	}
	if o.SizeNote != "" { // the size picker's rule (Host.MapSizeNote)
		ti.Warnings = append(ti.Warnings, o.SizeNote)
	}
	return ti
}

// nearbyAsked is how many bases to read when some may be razed.
const nearbyAsked = 12

// baseRows are the three nearest faction bases (WG-12): the first row
// carries the label.
func baseRows(p *planet.Planet, i int, live func(int) bool) []Row {
	tip := "Nearby bases\nThe closest faction settlements, with how many tiles away they are."
	bs := p.NearbyBases(i, 3)
	if live != nil {
		bs = slices.DeleteFunc(p.NearbyBases(i, nearbyAsked), func(b planet.Base) bool { return !live(b.Tile) })
		bs = bs[:min(len(bs), 3)]
	}
	if len(bs) == 0 {
		return []Row{{"Nearby bases", "None nearby", tip, uikit.Text}}
	}
	rows := make([]Row, len(bs))
	for k, b := range bs {
		label := ""
		if k == 0 {
			label = "Nearby bases"
		}
		rows[k] = Row{label, fmt.Sprintf("%s, %d tiles", baseName(p, b), b.Hops), tip, uikit.Text}
	}
	return rows
}

// baseName is "Name (Faction)".
func baseName(p *planet.Planet, b planet.Base) string {
	fname := ""
	if b.Faction >= 0 && b.Faction < len(p.Factions) {
		fname = p.Factions[b.Faction].Name
	}
	return fmt.Sprintf("%s (%s)", p.SettlementName(b.Tile), fname)
}

// quadrumSeason names the season of a day of the year at a site, with
// the game's own names (Climate.SeasonAt).
func quadrumSeason(s site.Site, day int) string {
	return s.Climate.SeasonAt(clock.Tick(day * clock.TicksPerDay))
}

func quadrums(p *planet.Planet, i int, s site.Site, u units.Temp) []Quadrum {
	temps := p.QuadrumTemps(i)
	out := make([]Quadrum, len(temps))
	for q, t := range temps {
		out[q] = Quadrum{Season: quadrumSeason(s, q*clock.DaysPerQuadrum+clock.DaysPerQuadrum/2), Temp: u.Whole(t)}
	}
	return out
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

// LandingDetail is the biome card's text: name, blurb, climate, ground,
// plants and wildlife, wrapped to cols characters.
func LandingDetail(b game.BiomeInfo, cols int, u units.Temp) []string {
	title := b.Name
	if b.Difficulty != "" {
		title += "  [" + b.Difficulty + "]"
	}
	out := []string{title, ""}
	out = append(out, uikit.WrapCols(b.Blurb, cols)...)
	out = append(out, "",
		fmt.Sprintf("Temperature  %s winter low, %s summer high", u.Whole(float64(b.WinterLow)), u.Whole(float64(b.SummerHigh))),
		fmt.Sprintf("Growing      about %d days a year", b.GrowingDays),
	)
	out = append(out, wrapLabeled("Ground", b.Ground, cols)...)
	out = append(out, wrapLabeled("Plants", b.Plants, cols)...)
	out = append(out, wrapLabeled("Wildlife", b.Wildlife, cols)...)
	return out
}

// wrapLabeled is "Label      a, b, c" wrapped under its label column.
func wrapLabeled(label string, items []string, cols int) []string {
	const lw = 13
	text := "none"
	if len(items) > 0 {
		text = strings.Join(items, ", ")
	}
	rows := uikit.WrapCols(text, cols-lw)
	for i := range rows {
		if i == 0 {
			rows[i] = fmt.Sprintf("%-*s%s", lw, label, rows[i])
		} else {
			rows[i] = strings.Repeat(" ", lw) + rows[i]
		}
	}
	return rows
}
