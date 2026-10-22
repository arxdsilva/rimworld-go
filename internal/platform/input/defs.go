package input

// Action ids. They are the keys of the stored bindings, so never rename
// one without a settings migration.
const (
	CamLeft  Action = "camera.left"
	CamRight Action = "camera.right"
	CamUp    Action = "camera.up"
	CamDown  Action = "camera.down"
	CamFast  Action = "camera.fast"
	ZoomIn   Action = "camera.zoom_in"
	ZoomOut  Action = "camera.zoom_out"

	Pause     Action = "time.pause"
	Speed1    Action = "time.speed1"
	Speed2    Action = "time.speed2"
	Speed3    Action = "time.speed3"
	SpeedUp   Action = "time.faster"
	SpeedDown Action = "time.slower"

	TabArchitect Action = "tab.architect"
	TabWork      Action = "tab.work"
	TabSchedule  Action = "tab.schedule"
	TabAssign    Action = "tab.assign"
	TabAnimals   Action = "tab.animals"
	TabWildlife  Action = "tab.wildlife"
	TabResearch  Action = "tab.research"
	TabQuests    Action = "tab.quests"
	TabWorld     Action = "tab.world"
	TabHistory   Action = "tab.history"

	Screenshot Action = "view.screenshot"
	HideUI     Action = "view.hide_ui"

	Cancel Action = "dialog.cancel"
	Accept Action = "dialog.accept"

	PrevColonist Action = "select.prev_colonist"
	NextColonist Action = "select.next_colonist"
	CycleCell    Action = "select.cycle_cell"
	SelectAdd    Action = "select.add"

	Draft  Action = "thing.draft"
	Forbid Action = "thing.forbid"
	Power  Action = "thing.power"

	Gizmo1  Action = "gizmo.1"
	Gizmo2  Action = "gizmo.2"
	Gizmo3  Action = "gizmo.3"
	Gizmo4  Action = "gizmo.4"
	Gizmo5  Action = "gizmo.5"
	Gizmo6  Action = "gizmo.6"
	Gizmo7  Action = "gizmo.7"
	Gizmo8  Action = "gizmo.8"
	Gizmo9  Action = "gizmo.9"
	Gizmo10 Action = "gizmo.10"
	Gizmo11 Action = "gizmo.11"
	Gizmo12 Action = "gizmo.12"

	ToolCancel      Action = "arch.cancel_tool"
	ToolDeconstruct Action = "arch.deconstruct_tool"
	RotateLeft      Action = "arch.rotate_left"
	RotateRight     Action = "arch.rotate_right"
	Constrain       Action = "arch.constrain"

	RoofOverlay Action = "overlay.roof"
	RoomStats   Action = "overlay.rooms"
	Beauty      Action = "overlay.beauty"
	Detail      Action = "overlay.detail"
	Search      Action = "overlay.search"

	Queue   Action = "orders.queue"
	Qty10   Action = "qty.x10"
	Qty100  Action = "qty.x100"
	Qty1000 Action = "qty.x1000"

	DevUltrafast Action = "dev.ultrafast"
	DevStep      Action = "dev.step"
	DevActions   Action = "dev.actions"
	DevLog       Action = "dev.log"
	DevInspector Action = "dev.inspector"
)

// Gizmos are the inspector command keys, in button order.
var Gizmos = []Action{Gizmo1, Gizmo2, Gizmo3, Gizmo4, Gizmo5, Gizmo6, Gizmo7, Gizmo8, Gizmo9, Gizmo10, Gizmo11, Gizmo12}

// NotYet is the reason shown for a shortcut whose system does not exist.
const NotYet = "not implemented yet"

func k(key string) Binding     { return Binding{Key: key} }
func shift(key string) Binding { return Binding{Key: key, Mods: ModShift} }
func keys(ks ...string) []Binding {
	out := make([]Binding, 0, len(ks))
	for _, s := range ks {
		out = append(out, k(s))
	}
	return out
}

func gizmo(n int, key string) Def {
	return Def{ID: Gizmos[n-1], Label: "Command " + itoa(n), Category: "Commands", Context: CtxMap, Defaults: keys(key)}
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// defs is the action table in display order.
var defs = []Def{
	{ID: CamLeft, Label: "Pan left", Category: "Camera", Context: CtxGlobal, Kind: Hold, Defaults: keys("A", "ArrowLeft")},
	{ID: CamRight, Label: "Pan right", Category: "Camera", Context: CtxGlobal, Kind: Hold, Defaults: keys("D", "ArrowRight")},
	{ID: CamUp, Label: "Pan up", Category: "Camera", Context: CtxGlobal, Kind: Hold, Defaults: keys("W", "ArrowUp")},
	{ID: CamDown, Label: "Pan down", Category: "Camera", Context: CtxGlobal, Kind: Hold, Defaults: keys("S", "ArrowDown")},
	{ID: CamFast, Label: "Fast pan (hold)", Category: "Camera", Context: CtxGlobal, Kind: Modifier, Fixed: true, Defaults: keys(KeyShift)},
	{ID: ZoomIn, Label: "Zoom in", Category: "Camera", Context: CtxGlobal, Defaults: keys("PageDown", "Equal")},
	{ID: ZoomOut, Label: "Zoom out", Category: "Camera", Context: CtxGlobal, Defaults: keys("PageUp", "Minus")},

	{ID: Pause, Label: "Pause / resume", Category: "Time", Context: CtxGlobal, Kind: Toggle, Defaults: keys("Space")},
	{ID: Speed1, Label: "Normal speed", Category: "Time", Context: CtxGlobal, Defaults: keys("Digit1")},
	{ID: Speed2, Label: "Fast speed", Category: "Time", Context: CtxGlobal, Defaults: keys("Digit2")},
	{ID: Speed3, Label: "Superfast speed", Category: "Time", Context: CtxGlobal, Defaults: keys("Digit3")},
	{ID: SpeedUp, Label: "Speed up", Category: "Time", Context: CtxGlobal, Defaults: keys("Home")},
	{ID: SpeedDown, Label: "Slow down", Category: "Time", Context: CtxGlobal, Defaults: keys("End")},

	{ID: TabArchitect, Label: "Architect", Category: "Tabs", Context: CtxGlobal, Kind: Toggle, Defaults: keys("Tab")},
	{ID: TabWork, Label: "Work", Category: "Tabs", Context: CtxGlobal, Kind: Toggle, Defaults: keys("F1")},
	{ID: TabSchedule, Label: "Schedule", Category: "Tabs", Context: CtxGlobal, Kind: Toggle, Defaults: keys("F2")},
	{ID: TabAssign, Label: "Assign", Category: "Tabs", Context: CtxGlobal, Kind: Toggle, Defaults: keys("F3"), Reserved: NotYet},
	{ID: TabAnimals, Label: "Animals", Category: "Tabs", Context: CtxGlobal, Kind: Toggle, Defaults: keys("F4")},
	{ID: TabWildlife, Label: "Wildlife", Category: "Tabs", Context: CtxGlobal, Kind: Toggle, Defaults: keys("F5")},
	{ID: TabResearch, Label: "Research", Category: "Tabs", Context: CtxGlobal, Kind: Toggle, Defaults: keys("F6")},
	{ID: TabQuests, Label: "Quests", Category: "Tabs", Context: CtxGlobal, Kind: Toggle, Defaults: keys("F7"), Reserved: NotYet},
	{ID: TabWorld, Label: "World", Category: "Tabs", Context: CtxGlobal, Kind: Toggle, Defaults: keys("F8"), Reserved: NotYet},
	{ID: TabHistory, Label: "History", Category: "Tabs", Context: CtxGlobal, Kind: Toggle, Defaults: keys("F9"), Reserved: NotYet},

	{ID: Screenshot, Label: "Screenshot", Category: "View", Context: CtxGlobal, Defaults: keys("F10")},
	{ID: HideUI, Label: "Hide interface", Category: "View", Context: CtxGlobal, Kind: Toggle, Defaults: keys("F11")},

	{ID: Cancel, Label: "Cancel / back / menu", Category: "Dialogs", Context: CtxGlobal, Defaults: keys("Escape")},
	{ID: Accept, Label: "Accept", Category: "Dialogs", Context: CtxGlobal, Defaults: keys("Enter")},

	{ID: PrevColonist, Label: "Previous colonist", Category: "Selection", Context: CtxMap, Defaults: keys("Comma")},
	{ID: NextColonist, Label: "Next colonist", Category: "Selection", Context: CtxMap, Defaults: keys("Period")},
	{ID: CycleCell, Label: "Next thing in the cell", Category: "Selection", Context: CtxMap, Defaults: keys("NumpadMultiply")},
	{ID: SelectAdd, Label: "Add to selection (click)", Category: "Selection", Context: CtxMap, Kind: Modifier, Fixed: true, Defaults: keys(KeyShift)},

	{ID: Draft, Label: "Draft / undraft", Category: "Selected", Context: CtxMap, Kind: Toggle, Defaults: keys("R")},
	{ID: Forbid, Label: "Forbid / allow", Category: "Selected", Context: CtxMap, Kind: Toggle, Defaults: keys("F")},
	{ID: Power, Label: "Power on / off", Category: "Selected", Context: CtxMap, Kind: Toggle, Defaults: keys("V"), Reserved: NotYet},

	gizmo(1, "B"), gizmo(2, "H"), gizmo(3, "Y"), gizmo(4, "N"), gizmo(5, "J"), gizmo(6, "U"),
	gizmo(7, "M"), gizmo(8, "K"), gizmo(9, "I"), gizmo(10, "L"), gizmo(11, "O"), gizmo(12, "P"),

	{ID: ToolCancel, Label: "Cancel tool", Category: "Architect", Context: CtxMap, Defaults: keys("C")},
	{ID: ToolDeconstruct, Label: "Deconstruct tool", Category: "Architect", Context: CtxMap, Defaults: keys("X")},
	{ID: RotateLeft, Label: "Rotate left / prev. shape", Category: "Architect", Context: CtxTool, Defaults: keys("Q")},
	{ID: RotateRight, Label: "Rotate right / next shape", Category: "Architect", Context: CtxTool, Defaults: keys("E")},
	{ID: Constrain, Label: "Square / circle (drag)", Category: "Architect", Context: CtxTool, Kind: Modifier, Fixed: true, Defaults: keys(KeyCtrl)},

	{ID: RoofOverlay, Label: "Roof overlay", Category: "Overlays", Context: CtxMap, Kind: Toggle, Defaults: []Binding{shift("R")}},
	{ID: RoomStats, Label: "Room stats", Category: "Overlays", Context: CtxMap, Kind: Toggle, Defaults: keys("G")},
	{ID: Beauty, Label: "Beauty", Category: "Overlays", Context: CtxMap, Kind: Toggle, Defaults: keys("T"), Reserved: NotYet},
	{ID: Detail, Label: "Detailed cell info (hold)", Category: "Overlays", Context: CtxMap, Kind: Modifier, Fixed: true, Defaults: keys(KeyAlt)},
	{ID: Search, Label: "Search", Category: "Overlays", Context: CtxMap, Defaults: keys("Z")},

	{ID: Queue, Label: "Queue order (click)", Category: "Orders", Context: CtxMap, Kind: Modifier, Fixed: true, Defaults: keys(KeyShift)},
	{ID: Qty10, Label: "Amount x10 (click)", Category: "Orders", Context: CtxPanel, Kind: Modifier, Fixed: true, Defaults: keys(KeyCtrl)},
	{ID: Qty100, Label: "Amount x100 (click)", Category: "Orders", Context: CtxPanel, Kind: Modifier, Fixed: true, Defaults: keys(KeyShift)},
	{ID: Qty1000, Label: "Amount x1000 (Ctrl+Shift)", Category: "Orders", Context: CtxPanel, Kind: Modifier, Fixed: true, Defaults: keys(KeyCtrl)},

	{ID: DevUltrafast, Label: "Ultrafast 15x", Category: "Developer", Context: CtxDev, Defaults: keys("Digit4")},
	{ID: DevStep, Label: "Step one tick (paused)", Category: "Developer", Context: CtxDev, Defaults: keys("NumpadAdd")},
	{ID: DevActions, Label: "Debug actions", Category: "Developer", Context: CtxDev, Kind: Toggle, Defaults: keys("Slash")},
	{ID: DevLog, Label: "Debug log", Category: "Developer", Context: CtxDev, Kind: Toggle, Defaults: keys("Backquote")},
	{ID: DevInspector, Label: "Debug inspector", Category: "Developer", Context: CtxDev, Kind: Toggle, Defaults: keys("Quote")},
}
