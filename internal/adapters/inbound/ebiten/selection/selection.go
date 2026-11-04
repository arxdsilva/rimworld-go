// Package selection is what the player has selected: one pawn, animal,
// item, plant, building, frame or zone, or a group of several, the click
// and box-select rules that set it, and the group's bookkeeping. It is the
// single owner of selection state, reached by the inspector, orders and
// HUD through Model (ARCH-005).
package selection

import (
	"slices"
	"strconv"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/pkg/mathx"
)

// hiddenConduitKind is the building kind drawn only while the power
// overlay shows (power_draw.go).
const hiddenConduitKind = "hidden conduit"

// Kind is the kind of thing selected.
type Kind uint8

const (
	None Kind = iota
	Pawn
	Animal
	Item
	Building
	Plant
	Frame
	Zone // PLAY-020: a zone (the id is a zone id)
	// Mixed is a group's kind when its members differ (CTL-006); never a Cand's.
	Mixed
)

// Cand is one selectable thing on a tile.
type Cand struct {
	Kind Kind
	ID   entity.EntityID
}

// Cycle remembers the last click, for cycling through a tile's stack.
type Cycle struct {
	Tile [2]int
	Idx  int
	N    int
	Ok   bool
}

// Group is a multi-selection. Its members may be of different kinds (Shift,
// CTL-006); Kind says which.
type Group struct {
	Members []Cand
}

// GroupOf is a group of ids that are all of one kind.
func GroupOf(kind Kind, ids []entity.EntityID) Group {
	g := Group{Members: make([]Cand, len(ids))}
	for i, id := range ids {
		g.Members[i] = Cand{Kind: kind, ID: id}
	}
	return g
}

// Kind is the members' common kind, Mixed when they differ, None when empty.
func (g Group) Kind() Kind {
	if len(g.Members) == 0 {
		return None
	}
	k := g.Members[0].Kind
	for _, c := range g.Members[1:] {
		if c.Kind != k {
			return Mixed
		}
	}
	return k
}

// IDs is every member's id, in selection order. Across kinds an id alone
// does not name a member: read Members for that.
func (g Group) IDs() []entity.EntityID {
	out := make([]entity.EntityID, len(g.Members))
	for i, c := range g.Members {
		out[i] = c.ID
	}
	return out
}

// IDsOf is the ids of the members of kind k, in selection order.
func (g Group) IDsOf(k Kind) []entity.EntityID {
	var out []entity.EntityID
	for _, c := range g.Members {
		if c.Kind == k {
			out = append(out, c.ID)
		}
	}
	return out
}

// Press is the map's left press: where it went down (for the drag box)
// and the last click (for double clicks).
type Press struct {
	On, Box, Shift bool
	X, Y           int
	LastAt         uint64 // the Update counter of the last map click
	LastX, LastY   int
}

const (
	BoxMinPx          = 6  // drag this far before a press becomes a box
	DoubleClickFrames = 18 // two clicks this close (Update frames) are a double click
	DoubleClickPx     = 6
)

// Model holds the selection. The fields are exported for the root's
// panels while they still read them; every change goes through a method.
type Model struct {
	sh   Shell
	host Host

	SelBuilding, SelAnimal, SelPlant, SelItem, SelFrame entity.EntityID
	HasBuilding, HasAnimal, HasPlant, HasItem, HasFrame bool
	ZoneSel                                             uint16 // the selected zone's id
	HasZone                                             bool

	Cycle Cycle // last map click, for cycling a tile's stack
	Group Group // multi-selection
	Press Press // the map left press: drag box and double click
}

// New is an empty selection reading the colony through sh.
func New(sh Shell, host Host) *Model { return &Model{sh: sh, host: host} }

// Grouped reports a multi-selection is active.
func (m *Model) Grouped() bool { return len(m.Group.Members) >= 2 }

// InGroup reports id is a member of a group of kind k.
func (m *Model) InGroup(k Kind, id entity.EntityID) bool {
	return m.Grouped() && slices.Contains(m.Group.Members, Cand{Kind: k, ID: id})
}

// Selectables is SelectablesAt for a click on the map: an animal whose
// drawn body covers the tile from another cell (PLAY-022 C7) comes after
// whatever stands on the tile itself and before its plants, floors and
// zone; a hidden conduit counts only while the power overlay shows.
func (m *Model) Selectables(w game.WorldView, tx, ty int) []Cand {
	cands, solid := SelectablesSplit(w, m.sh.Pawns(), tx, ty)
	if !fogged(w, tx, ty) {
		if body := m.host.AnimalsCovering(w, tx, ty); len(body) > 0 {
			cands = slices.Insert(cands, solid, body...)
		}
	}
	if m.host.PowerShown() {
		return cands
	}
	return slices.DeleteFunc(cands, func(c Cand) bool {
		if c.Kind != Building {
			return false
		}
		for i := range w.Buildings {
			if w.Buildings[i].ID == c.ID {
				return w.Buildings[i].Kind == hiddenConduitKind
			}
		}
		return false
	})
}

// SelectablesAt lists what a click on (tx, ty) can select, in precedence
// pawn, animal, ground item, frame, building, plant, floor, zone.
func SelectablesAt(w game.WorldView, pawns []game.PawnView, tx, ty int) []Cand {
	out, _ := SelectablesSplit(w, pawns, tx, ty)
	return out
}

// SelectablesSplit is SelectablesAt and solid, the index where the things
// standing on the tile end and its ground (plants, floors, zone) begins.
func SelectablesSplit(w game.WorldView, pawns []game.PawnView, tx, ty int) (out []Cand, solid int) {
	if fogged(w, tx, ty) {
		return nil, 0 // unexplored: nothing to pick
	}
	for _, p := range pawns {
		if p.X == tx && p.Y == ty && !p.Embodied {
			out = append(out, Cand{Kind: Pawn, ID: p.ID})
		}
	}
	for _, an := range w.Animals {
		if an.X == tx && an.Y == ty {
			out = append(out, Cand{Kind: Animal, ID: an.ID})
		}
	}
	for _, it := range w.Items {
		if it.X == tx && it.Y == ty && !it.Carried {
			out = append(out, Cand{Kind: Item, ID: it.ID})
		}
	}
	var floor, floorFrames []Cand
	for _, f := range w.Frames {
		fw, fh := max(f.W, 1), max(f.H, 1)
		if tx < f.X || ty < f.Y || tx >= f.X+fw || ty >= f.Y+fh {
			continue
		}
		if f.Floor {
			floorFrames = append(floorFrames, Cand{Kind: Frame, ID: f.ID})
		} else {
			out = append(out, Cand{Kind: Frame, ID: f.ID})
		}
	}
	for _, b := range w.Buildings {
		bw, bh := max(b.W, 1), max(b.H, 1)
		if tx < b.X || ty < b.Y || tx >= b.X+bw || ty >= b.Y+bh {
			continue
		}
		if b.Floor {
			floor = append(floor, Cand{Kind: Building, ID: b.ID})
		} else {
			out = append(out, Cand{Kind: Building, ID: b.ID})
		}
	}
	solid = len(out)
	for _, p := range w.Plants {
		if p.X == tx && p.Y == ty {
			out = append(out, Cand{Kind: Plant, ID: p.ID})
		}
	}
	out = append(out, floor...)
	out = append(out, floorFrames...)
	if z, ok := zoneCellAt(w, tx, ty); ok { // PLAY-020: a zone comes last
		out = append(out, Cand{Kind: Zone, ID: entity.EntityID(z.ID)})
	}
	return out, solid
}

// ClickSelect selects what is under the cursor (a map left click).
func (m *Model) ClickSelect() {
	mx, my := m.sh.MapCursor()
	tx, ty := m.sh.TileAt(mx, my)
	w := m.sh.World()
	if m.host.SelectZoneLabel(w, mx, my, tx, ty) { // a click on a zone's name plate
		m.Cycle = Cycle{}
		return
	}
	cands := m.Selectables(w, tx, ty)
	tile := [2]int{tx, ty}
	m.host.NoteCell(tile)
	if c := m.Cycle; c.Ok && c.Tile == tile && c.N == len(cands) && len(cands) > 1 {
		i := (c.Idx + 1) % len(cands)
		m.Select(cands[i])
		m.Cycle.Idx = i
		return
	}
	if id, ok := m.host.PawnUnderCursor(mx, my); ok {
		m.Select(Cand{Kind: Pawn, ID: id})
		m.Cycle = Cycle{Tile: tile, N: len(cands), Ok: true}
		for i, c := range cands {
			if c.Kind == Pawn && c.ID == id {
				m.Cycle.Idx = i
			}
		}
		return
	}
	if len(cands) == 0 {
		m.Clear(None)
		m.Cycle = Cycle{}
		return
	}
	m.Select(cands[0])
	m.Cycle = Cycle{Tile: tile, N: len(cands), Ok: true}
}

// Select selects c and clears every other kind of selection.
func (m *Model) Select(c Cand) {
	m.Clear(c.Kind)
	switch c.Kind {
	case Pawn:
		m.sh.Select(c.ID)
	case Animal:
		m.SelAnimal, m.HasAnimal = c.ID, true
	case Item:
		m.SelItem, m.HasItem = c.ID, true
	case Building:
		m.SelBuilding, m.HasBuilding = c.ID, true
	case Plant:
		m.SelPlant, m.HasPlant = c.ID, true
	case Frame:
		m.SelFrame, m.HasFrame = c.ID, true
	case Zone:
		m.ZoneSel, m.HasZone = uint16(c.ID), true
		m.host.SetZone(uint16(c.ID), true)
	}
}

// Clear drops every selection but kind keep, and any group.
func (m *Model) Clear(keep Kind) {
	m.Group = Group{}
	if keep != Pawn {
		m.sh.ClearSelect()
	}
	if keep != Animal {
		m.HasAnimal = false
	}
	if keep != Item {
		m.HasItem = false
	}
	if keep != Building {
		m.HasBuilding = false
	}
	if keep != Plant {
		m.HasPlant = false
	}
	if keep != Frame {
		m.HasFrame = false
	}
	if keep != Zone {
		m.ClearZone()
	}
}

// ClearZone drops the zone selection and ends its Expand and Shrink tool.
func (m *Model) ClearZone() {
	m.HasZone, m.ZoneSel = false, 0
	m.host.SetZone(0, false)
}

// SelectPlantAt selects the plant at (tx, ty).
func (m *Model) SelectPlantAt(tx, ty int) bool {
	for _, p := range m.sh.World().Plants {
		if p.X == tx && p.Y == ty {
			m.Select(Cand{Kind: Plant, ID: p.ID})
			return true
		}
	}
	return false
}

// SelectItemAt selects the ground item at (tx, ty).
func (m *Model) SelectItemAt(tx, ty int) bool {
	for _, it := range m.sh.World().Items {
		if it.X == tx && it.Y == ty && !it.Carried {
			m.Select(Cand{Kind: Item, ID: it.ID})
			return true
		}
	}
	return false
}

// ClearPlant and ClearItem drop just that selection.
func (m *Model) ClearPlant() { m.HasPlant = false }
func (m *Model) ClearItem()  { m.HasItem = false }

// Selected is the single selection as a candidate.
func (m *Model) Selected() (Cand, bool) {
	switch id, pawn := m.sh.Selected(); {
	case pawn:
		return Cand{Kind: Pawn, ID: id}, true
	case m.HasAnimal:
		return Cand{Kind: Animal, ID: m.SelAnimal}, true
	case m.HasItem:
		return Cand{Kind: Item, ID: m.SelItem}, true
	case m.HasBuilding:
		return Cand{Kind: Building, ID: m.SelBuilding}, true
	case m.HasPlant:
		return Cand{Kind: Plant, ID: m.SelPlant}, true
	case m.HasFrame:
		return Cand{Kind: Frame, ID: m.SelFrame}, true
	case m.HasZone:
		return Cand{Kind: Zone, ID: entity.EntityID(m.ZoneSel)}, true
	}
	return Cand{}, false
}

// SelectedPlant is the selected plant, if it still exists.
func (m *Model) SelectedPlant() (game.PlantView, bool) {
	if !m.HasPlant {
		return game.PlantView{}, false
	}
	for _, p := range m.sh.World().Plants {
		if p.ID == m.SelPlant {
			return p, true
		}
	}
	m.HasPlant = false // cut, harvested or eaten
	return game.PlantView{}, false
}

// SelectedItem is the selected ground item, if it is still on the ground.
func (m *Model) SelectedItem() (game.ItemView, bool) {
	if !m.HasItem {
		return game.ItemView{}, false
	}
	for _, it := range m.sh.World().Items {
		if it.ID == m.SelItem && !it.Carried {
			return it, true
		}
	}
	m.HasItem = false // picked up, used or rotted away
	return game.ItemView{}, false
}

// SelectedAnimal is the selected animal, if it is still there.
func (m *Model) SelectedAnimal() (game.AnimalView, bool) {
	if !m.HasAnimal {
		return game.AnimalView{}, false
	}
	for _, an := range m.sh.World().Animals {
		if an.ID == m.SelAnimal {
			return an, true
		}
	}
	m.HasAnimal = false // it died or left
	return game.AnimalView{}, false
}

// SelectedFrame is the selected blueprint or frame, if it still exists.
func (m *Model) SelectedFrame() (game.FrameView, bool) {
	if !m.HasFrame {
		return game.FrameView{}, false
	}
	for _, f := range m.sh.World().Frames {
		if f.ID == m.SelFrame {
			return f, true
		}
	}
	m.HasFrame = false // finished, cancelled or removed
	return game.FrameView{}, false
}

// SetGroup selects ids of kind: nothing, one thing (the single panel) or
// a group.
func (m *Model) SetGroup(kind Kind, ids []entity.EntityID) {
	m.SetMembers(GroupOf(kind, ids).Members)
}

// SetMembers selects the members, of any kinds: nothing, one thing (the
// single panel) or a group.
func (m *Model) SetMembers(members []Cand) {
	var clean []Cand
	seen := map[Cand]bool{}
	for _, c := range members {
		if !seen[c] {
			seen[c] = true
			clean = append(clean, c)
		}
	}
	switch len(clean) {
	case 0:
		m.Clear(None)
	case 1:
		m.Select(clean[0])
	default:
		m.Clear(None)
		m.Group = Group{Members: clean}
	}
}

// SelectionSet is the current selection as a kind and ids (a group or the
// single selection of a kind that can group). Across kinds, Group.Members
// names each member.
func (m *Model) SelectionSet() (Kind, []entity.EntityID) {
	g := Group{Members: m.currentSet()}
	return g.Kind(), g.IDs()
}

// currentSet is a copy of the group, or the single selection if it can group.
func (m *Model) currentSet() []Cand {
	if m.Grouped() {
		return slices.Clone(m.Group.Members)
	}
	if c, ok := m.Selected(); ok && m.CanGroup(c) {
		return []Cand{c}
	}
	return nil
}

// ToggleInSet adds c to the selection, whatever kinds it holds, or takes it
// out.
func (m *Model) ToggleInSet(c Cand) {
	set := m.currentSet()
	if i := slices.Index(set, c); i >= 0 {
		m.SetMembers(slices.Delete(set, i, i+1))
		return
	}
	m.SetMembers(append(set, c))
}

// CandUnderCursor is what a plain click would select first.
func (m *Model) CandUnderCursor() (Cand, bool) {
	mx, my := m.sh.MapCursor()
	if id, ok := m.host.PawnUnderCursor(mx, my); ok {
		return Cand{Kind: Pawn, ID: id}, true
	}
	tx, ty := m.sh.TileAt(mx, my)
	if cands := m.Selectables(m.sh.World(), tx, ty); len(cands) > 0 {
		return cands[0], true
	}
	return Cand{}, false
}

// ShiftClick adds the thing under the cursor to the selection, or takes
// it out when it is already in. What cannot group takes a plain click.
func (m *Model) ShiftClick() {
	c, ok := m.CandUnderCursor()
	if !ok { // bare ground: the selection stays, for a Shift+box starting here
		return
	}
	if !m.CanGroup(c) {
		m.ClickSelect()
		return
	}
	m.ToggleInSet(c)
}

// ApplyBox selects what a dragged box covered (kind and ids from the
// renderer's hit test); with add (Shift) it joins the selection, whatever
// kinds that holds.
func (m *Model) ApplyBox(kind Kind, ids []entity.EntityID, add bool) {
	if kind == None {
		if !add {
			m.Clear(None)
		}
		return
	}
	boxed := GroupOf(kind, ids).Members
	if add {
		boxed = append(m.currentSet(), boxed...)
	}
	m.SetMembers(boxed)
}

// SelectedPawns is the selected colonists: the group's living members in
// selection order, or the single selected colonist.
func (m *Model) SelectedPawns() []game.PawnView {
	pawns := m.sh.Pawns()
	if !m.Grouped() {
		if id, ok := m.sh.Selected(); ok {
			for _, p := range pawns {
				if p.ID == id {
					return []game.PawnView{p}
				}
			}
		}
		return nil
	}
	return members(m.Group.IDsOf(Pawn), pawns, func(p game.PawnView) entity.EntityID { return p.ID })
}

// SelectedItems is the selected ground stacks (group or single).
func (m *Model) SelectedItems() []game.ItemView {
	if !m.Grouped() {
		if it, ok := m.SelectedItem(); ok {
			return []game.ItemView{it}
		}
		return nil
	}
	ids := m.Group.IDsOf(Item)
	if len(ids) == 0 { // the group marks ask every frame
		return nil
	}
	ground := slices.DeleteFunc(slices.Clone(m.sh.World().Items), func(it game.ItemView) bool { return it.Carried })
	return members(ids, ground, func(it game.ItemView) entity.EntityID { return it.ID })
}

// SelectedAnimals is the group's animals.
func (m *Model) SelectedAnimals() []game.AnimalView {
	if !m.Grouped() {
		return nil
	}
	return members(m.Group.IDsOf(Animal), m.sh.World().Animals, func(an game.AnimalView) entity.EntityID { return an.ID })
}

// SelectedBuildings is the group's buildings.
func (m *Model) SelectedBuildings() []game.BuildingView {
	if !m.Grouped() {
		return nil
	}
	return members(m.Group.IDsOf(Building), m.sh.World().Buildings, func(b game.BuildingView) entity.EntityID { return b.ID })
}

// SelectedFrames is the group's blueprints.
func (m *Model) SelectedFrames() []game.FrameView {
	if !m.Grouped() {
		return nil
	}
	return members(m.Group.IDsOf(Frame), m.sh.World().Frames, func(f game.FrameView) entity.EntityID { return f.ID })
}

// SelectedZones is the group's zones; a member's ID is the zone id.
func (m *Model) SelectedZones() []game.ZoneView {
	if !m.Grouped() {
		return nil
	}
	return members(m.Group.IDsOf(Zone), m.sh.World().Zones, func(z game.ZoneView) entity.EntityID { return entity.EntityID(z.ID) })
}

// members returns the views for ids, in id order, dropping ones that are
// gone. The group marks ask every frame, so no ids reads nothing and only
// the ids get a map entry, never the views.
func members[V any](ids []entity.EntityID, views []V, id func(V) entity.EntityID) []V {
	if len(ids) == 0 {
		return nil
	}
	at := make(map[entity.EntityID]int, len(ids))
	for _, i := range ids {
		at[i] = -1
	}
	for vi := range views {
		k := id(views[vi])
		if _, want := at[k]; want {
			at[k] = vi
		}
	}
	var out []V
	for _, i := range ids {
		if vi := at[i]; vi >= 0 {
			out = append(out, views[vi])
		}
	}
	return out
}

// HandleMapMouse is the map's left button (select, Shift-toggle, box,
// double click) and then the right button (orders). thingsIn lists what a
// finished box covers; sameOnScreen is the double click.
func (m *Model) HandleMapMouse(f Mouse, sameOnScreen func(), box func(x0, y0, x1, y1 int, add bool)) {
	mx, my := m.sh.MapCursor() // the press, the box and the double click live in map pixels
	updates, k := m.host.Frame()
	boxMin, dblPx := int(BoxMinPx*k), int(DoubleClickPx*k)
	switch {
	case f.Clicked():
		f.Take()
		p := m.Press
		double := p.LastAt > 0 && updates-p.LastAt <= DoubleClickFrames &&
			mathx.Abs(mx-p.LastX) <= dblPx && mathx.Abs(my-p.LastY) <= dblPx
		m.Press = Press{On: true, X: mx, Y: my, Shift: f.Shift(), LastAt: updates, LastX: mx, LastY: my}
		switch {
		case double:
			m.Press.LastAt = 0 // a third click starts over
			sameOnScreen()
		case f.Shift():
			m.ShiftClick()
		default:
			m.ClickSelect()
		}
	case m.Press.On && f.Down():
		if !m.Press.Box && (mathx.Abs(mx-m.Press.X) > boxMin || mathx.Abs(my-m.Press.Y) > boxMin) {
			m.Press.Box = true
		}
	case m.Press.On:
		if m.Press.Box {
			box(m.Press.X, m.Press.Y, mx, my, m.Press.Shift)
		}
		m.Press.On, m.Press.Box = false, false
	}
	m.host.HandleOrders()
}

// Mouse is the map's left button for one frame.
type Mouse interface {
	Clicked() bool
	Take()
	Down() bool
	Shift() bool
}

// ApplyGroupClick handles a "group.*" button that acts on the members.
func (m *Model) ApplyGroupClick(id string) {
	switch id {
	case "group.close":
		m.Clear(None)
	case "group.draft", "group.undraft":
		on := id == "group.draft"
		for _, p := range m.SelectedPawns() {
			if p.Drafted != on {
				m.sh.Enqueue(command.Command{Kind: command.DraftPawn, Pawn: uint64(p.ID), Enabled: on})
			}
		}
	case "group.forbid", "group.allow":
		on := id == "group.forbid"
		for _, it := range m.SelectedItems() {
			if it.Forbidden != on {
				m.sh.Enqueue(command.Command{Kind: command.SetForbidden, Target: uint64(it.ID), Enabled: on})
			}
		}
	case "group.haulmark":
		for _, it := range m.SelectedItems() {
			if it.Unclaimed && !it.HaulMarked {
				m.sh.Enqueue(command.Command{Kind: command.SetHaulMark, Target: uint64(it.ID), Enabled: true})
			}
		}
	default:
		if c, ok := m.pickMember(id); ok {
			m.Select(c)
		}
	}
}

// PickID is the button id of a group row. A zone's id can equal another
// member's entity id, so a zone's carries a mark.
func PickID(c Cand) string {
	n := strconv.FormatUint(uint64(c.ID), 10)
	if c.Kind == Zone {
		return "group.pick.z" + n
	}
	return "group.pick." + n
}

// pickMember is the group member a pick button names. An unmarked id means
// the entity with that id, then the zone.
func (m *Model) pickMember(id string) (Cand, bool) {
	rest, ok := strings.CutPrefix(id, "group.pick.")
	if !ok {
		rest = id
	}
	rest, isZone := strings.CutPrefix(rest, "z")
	n, err := strconv.ParseUint(rest, 10, 64)
	if err != nil {
		return Cand{}, false
	}
	zone := Cand{Kind: Zone, ID: entity.EntityID(n)}
	if !isZone {
		for _, c := range m.Group.Members {
			if c.ID == zone.ID && c.Kind != Zone {
				return c, true
			}
		}
	}
	return zone, slices.Contains(m.Group.Members, zone)
}
