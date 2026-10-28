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

// Group is a multi-selection of one kind (colonists, animals or items).
type Group struct {
	Kind Kind
	Ids  []entity.EntityID
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

// Groupable kinds can form a group.
func Groupable(k Kind) bool { return k == Pawn || k == Animal || k == Item }

// Grouped reports a multi-selection is active.
func (m *Model) Grouped() bool { return len(m.Group.Ids) >= 2 }

// InGroup reports id is a member of a group of kind k.
func (m *Model) InGroup(k Kind, id entity.EntityID) bool {
	return m.Grouped() && m.Group.Kind == k && slices.Contains(m.Group.Ids, id)
}

// Selectables is SelectablesAt for a click on the map: an animal whose
// drawn body covers the tile from another cell (PLAY-022 C7) comes after
// whatever stands on the tile itself and before its plants, floors and
// zone; a hidden conduit counts only while the power overlay shows.
func (m *Model) Selectables(w game.WorldView, tx, ty int) []Cand {
	cands, solid := selectablesSplit(w, m.sh.Pawns(), tx, ty)
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
	out, _ := selectablesSplit(w, pawns, tx, ty)
	return out
}

// selectablesSplit is SelectablesAt and solid, the index where the things
// standing on the tile end and its ground (plants, floors, zone) begins.
func selectablesSplit(w game.WorldView, pawns []game.PawnView, tx, ty int) (out []Cand, solid int) {
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
	var clean []entity.EntityID
	seen := map[entity.EntityID]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			clean = append(clean, id)
		}
	}
	switch len(clean) {
	case 0:
		m.Clear(None)
	case 1:
		m.Select(Cand{Kind: kind, ID: clean[0]})
	default:
		m.Clear(None)
		m.Group = Group{Kind: kind, Ids: clean}
	}
}

// SelectionSet is the current selection as a kind and ids (a group or the
// single selection of a groupable kind).
func (m *Model) SelectionSet() (Kind, []entity.EntityID) {
	if m.Grouped() {
		return m.Group.Kind, slices.Clone(m.Group.Ids)
	}
	if c, ok := m.Selected(); ok && Groupable(c.Kind) {
		return c.Kind, []entity.EntityID{c.ID}
	}
	return None, nil
}

// ToggleInSet adds c to the selection or takes it out (a different kind
// starts over with c).
func (m *Model) ToggleInSet(c Cand) {
	kind, ids := m.SelectionSet()
	if kind != c.Kind {
		m.SetGroup(c.Kind, []entity.EntityID{c.ID})
		return
	}
	out := ids[:0]
	found := false
	for _, id := range ids {
		if id == c.ID {
			found = true
			continue
		}
		out = append(out, id)
	}
	if !found {
		out = append(out, c.ID)
	}
	m.SetGroup(kind, out)
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
// it out when it is already in. Other kinds start a new selection.
func (m *Model) ShiftClick() {
	c, ok := m.CandUnderCursor()
	if !ok || !Groupable(c.Kind) {
		m.ClickSelect()
		return
	}
	m.ToggleInSet(c)
}

// ApplyBox selects what a dragged box covered (kind and ids from the
// renderer's hit test); with add (Shift) it joins a selection of the same
// kind.
func (m *Model) ApplyBox(kind Kind, ids []entity.EntityID, add bool) {
	if kind == None {
		if !add {
			m.Clear(None)
		}
		return
	}
	if add {
		if k, cur := m.SelectionSet(); k == kind {
			ids = append(cur, ids...)
		}
	}
	m.SetGroup(kind, ids)
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
	if m.Group.Kind != Pawn {
		return nil
	}
	return members(m.Group.Ids, pawns, func(p game.PawnView) entity.EntityID { return p.ID })
}

// SelectedItems is the selected ground stacks (group or single).
func (m *Model) SelectedItems() []game.ItemView {
	if !m.Grouped() {
		if it, ok := m.SelectedItem(); ok {
			return []game.ItemView{it}
		}
		return nil
	}
	if m.Group.Kind != Item {
		return nil
	}
	ground := slices.DeleteFunc(slices.Clone(m.sh.World().Items), func(it game.ItemView) bool { return it.Carried })
	return members(m.Group.Ids, ground, func(it game.ItemView) entity.EntityID { return it.ID })
}

// SelectedAnimals is the group's animals.
func (m *Model) SelectedAnimals() []game.AnimalView {
	if !m.Grouped() || m.Group.Kind != Animal {
		return nil
	}
	return members(m.Group.Ids, m.sh.World().Animals, func(an game.AnimalView) entity.EntityID { return an.ID })
}

// members returns the views for ids, in id order, dropping ones that are
// gone.
func members[V any](ids []entity.EntityID, views []V, id func(V) entity.EntityID) []V {
	byID := make(map[entity.EntityID]V, len(views))
	for _, v := range views {
		byID[id(v)] = v
	}
	var out []V
	for _, i := range ids {
		if v, ok := byID[i]; ok {
			out = append(out, v)
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
			abs(mx-p.LastX) <= dblPx && abs(my-p.LastY) <= dblPx
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
		if !m.Press.Box && (abs(mx-m.Press.X) > boxMin || abs(my-m.Press.Y) > boxMin) {
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

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
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
		if n, ok := pickID(id); ok {
			m.Select(Cand{Kind: m.Group.Kind, ID: n})
		}
	}
}

func pickID(id string) (entity.EntityID, bool) {
	rest, ok := strings.CutPrefix(id, "group.pick.")
	if !ok {
		rest = id
	}
	n, err := strconv.ParseUint(rest, 10, 64)
	return entity.EntityID(n), err == nil
}
