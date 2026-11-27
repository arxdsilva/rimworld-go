// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package scripting

// Bounds of the extension points' values.
const (
	MaxQueryRows  = 256 // rows one ctx query returns
	MaxPanelLines = 12  // lines in one panel
	MaxActions    = 4   // actions on one panel
	// OrderEvery is the cadence visits between two script orders of one
	// pawn: a pawn takes a ScriptOrder only on the visit where
	// (tick / PawnEvery) % OrderEvery == 0, so at most one per
	// OrderEvery*PawnEvery ticks, derived from the tick alone.
	OrderEvery = 10
)

// EventKind is a game event a plugin can subscribe to (EVENTS) and get
// in on_event. The zero value is no event.
type EventKind uint8

// The events, by the name a script lists in EVENTS.
const (
	Quadrum      EventKind = iota + 1 // a quadrum began
	Year                              // a year began
	ResearchDone                      // a research project finished (Event.Research)
	RecipeDone                        // a bill's craft finished (Event.Recipe)
	MapGenerated                      // a new game's home map was generated, before its first tick (Event.Map)
)

// eventNames are the events' script names, by value.
var eventNames = [...]string{
	Quadrum: "quadrum", Year: "year", ResearchDone: "research_done",
	RecipeDone: "recipe_done", MapGenerated: "map_generated",
}

// String is the event's script name; "" for no event.
func (k EventKind) String() string {
	if int(k) >= len(eventNames) {
		return ""
	}
	return eventNames[k]
}

// EventKinds lists every event in value order.
func EventKinds() []EventKind {
	return []EventKind{Quadrum, Year, ResearchDone, RecipeDone, MapGenerated}
}

// ParseEventKind finds an event by its script name.
func ParseEventKind(name string) (EventKind, bool) {
	for k, n := range eventNames {
		if n != "" && n == name {
			return EventKind(k), true
		}
	}
	return 0, false
}

// Craft is RecipeDone's finished craft: Count of Product by recipe Key,
// made by colonist Worker at the bench on X, Y. (The contract's
// RecipeDone struct; renamed, since RecipeDone is the EventKind.)
type Craft struct {
	Key, Product string
	Count        int
	Worker       uint64
	X, Y         int
}

// MapGen is a generated map: its seed, size and biome key.
type MapGen struct {
	Seed  uint64
	W, H  int
	Biome string
}

// CommandView is a player command as validate_command sees it: Kind is
// one of build, add_bill, research, prioritize, draft.
type CommandView struct {
	Kind string
	Pawn uint64
	X, Y int
	Name string
}

// Panel is one plugin's panel: read-only text a script returns hourly.
type Panel struct {
	Plugin, Title string
	Lines         []string // at most MaxPanelLines
	Actions       []PanelAction
}

// PanelAction is a button on a panel; pressing it sends a ScriptAction.
type PanelAction struct {
	ID, Label string
}

// Animal is one wild species on the home map.
type Animal struct {
	Species, Label string
	Count          int
	Predator       bool
}

// Faction is one faction of the world.
type Faction struct {
	Name     string
	Goodwill int
	Hostile  bool
}

// Site is one world site.
type Site struct {
	Name, Faction string
}

// BuildingCount is how many buildings of one kind stand on the home map.
type BuildingCount struct {
	Kind  string
	Count int
}

// EventSink delivers game events to the plugins subscribed to them. The
// simulation owner calls it, never concurrently.
type EventSink interface {
	// Subscribed reports that a live script takes k and may; cheap, no
	// allocation.
	Subscribed(k EventKind) bool
	// Event runs every subscribed script's on_event for ev.Kind.
	Event(ev Event, w World, p PawnReader) Result
}

// Behaviour lets plugins replace incidents and veto player commands.
type Behaviour interface {
	// RunsIncidents reports that a live script defines run_incident.
	RunsIncidents() bool
	// RunIncident runs ev.Incident's plugin's run_incident; replaced is
	// true when it took the incident's place.
	RunIncident(ev Event, w World, p PawnReader) (res Result, replaced bool)
	// Validates reports that a live script defines validate_command.
	Validates() bool
	// ValidateCommand asks every validating script about c; a non-empty
	// reason from plugin drops the command.
	ValidateCommand(c CommandView, ev Event, w World, p PawnReader) (plugin, reason string, res Result)
}

// PanelSource gives the plugins' panels and runs their actions.
type PanelSource interface {
	// HasPanels reports that a live script defines panel.
	HasPanels() bool
	// Panels runs every panel hook, plugins in load order.
	Panels(ev Event, w World, p PawnReader) ([]Panel, Result)
	// Act runs ev.Plugin's on_action for ev.Action.
	Act(ev Event, w World, p PawnReader) Result
}

// Extensions is every run-time extension point the game calls beside
// Host.
type Extensions interface {
	EventSink
	Behaviour
	PanelSource
}

// AnimalReader reads the home map's wild animals.
type AnimalReader interface {
	Animals() []Animal // at most MaxQueryRows, species ascending
}

// MapReader reads the home map.
type MapReader interface {
	Size() (w, h int)
	// Terrain is the cell's terrain key; ok is false off the map.
	Terrain(x, y int) (string, bool)
	// Temperature is the cell's temperature in whole degrees C; ok is
	// false off the map.
	Temperature(x, y int) (int, bool)
	Weather() string
}

// WorldReader reads the world.
type WorldReader interface {
	Factions() []Faction // at most MaxQueryRows
	Sites() []Site       // at most MaxQueryRows
}

// BuildingReader reads the home map's buildings.
type BuildingReader interface {
	Buildings() []BuildingCount // at most MaxQueryRows, kind ascending
}

// Queries are the readers the world capability's ctx calls read. The game
// answers for its home map during a call only. A nil reader reads as
// empty: no rows, a 0x0 map, no weather.
type Queries struct {
	Animals   AnimalReader
	Map       MapReader
	World     WorldReader
	Buildings BuildingReader
}
