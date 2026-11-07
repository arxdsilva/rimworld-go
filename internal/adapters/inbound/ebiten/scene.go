package ebiten

import (
	camerapkg "rimworld-go/internal/adapters/inbound/ebiten/camera"
	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	creaturepkg "rimworld-go/internal/adapters/inbound/ebiten/render/creature"
	envpkg "rimworld-go/internal/adapters/inbound/ebiten/render/env"
	minimappkg "rimworld-go/internal/adapters/inbound/ebiten/render/minimap"
	motionpkg "rimworld-go/internal/adapters/inbound/ebiten/render/motion"
	"rimworld-go/internal/adapters/inbound/ebiten/render/world"

	"rimworld-go/internal/adapters/inbound/ebiten/render"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// One scene per open map (MAP-1). The simulation reports one map at a time
// (game.ViewedMap); everything the renderer remembers about that map lives
// in its mapScene, which the Renderer embeds, so r.cam, r.mo, r.wp … are
// the viewed map's. Each scene owns its world pipeline, and the pipeline
// points at the scene's own fields (layers.go: pipe), so the chunk, snow
// and minimap caches never mix maps. What follows the player rather than a
// map stays on the Renderer: the speed, selection, options, the eased pan
// (camera.Smooth carries the pan-speed and Reduced Motion settings), the
// sound-cue bus, the lighting shader with its pair cache (the light grid
// restarts on a switch) and the water shader.

// mapScene is one map's presentation state. It has no methods: the
// Renderer reads it through the embedding and sceneSet moves it.
type mapScene struct {
	cam camerapkg.Camera // this map's view (VIEW-001 saves one per map)
	// last is each pawn's tile walk; mo the per-creature motion tracks and
	// the animT clock (render/motion), each track carrying its animal's
	// body player. Both are cleared when the map is entered again.
	last map[entity.EntityID]pawnVisual
	mo   motionpkg.State[creaturepkg.Body]
	// parts, floaters and itemBorn are the map's particles, floating
	// numbers and item drop-in stamps (render/creature).
	parts    []creaturepkg.Particle
	floaters []creaturepkg.Floater
	itemBorn map[entity.EntityID]uint64
	split    pawnSplit // the map's snapshot split into colonists and raiders
	env      envState  // wind, contact springs, weather and shadows (environment.go)
	// wp is the world draw order and every draw layer, the zoom state with
	// its chunk and map bakes, the link grid and the stone cache
	// (render/world, layers.go: pipe); nil until first drawn, and again
	// once the scene went cold (sceneSet.chill).
	wp     *world.Pipeline
	snowc  envpkg.SnowCache // snow veil and wet masks per chunk (snow_cache.go)
	treeFx envpkg.TreeFx    // see-through tree canopies (tree_fade.go)
	view   render.View      // the pipeline's frame context (render/view.go)
	// figureBlood, while non-zero, is the blood stain drawLit puts on the
	// figure layer being drawn (lit.kage Custom3, blood_draw.go); the
	// pipeline's shared scratch.
	figureBlood float32
	heard       game.WorldView // last advance's world, which the sound cues compare with
	mini        minimappkg.Map // the map's minimap (render/minimap)
}

// newMapScene is a scene never shown: its camera is unplaced, so the first
// Clamp centres it on the map (or its landing spot) as for a new colony.
func newMapScene() *mapScene {
	return &mapScene{cam: camerapkg.Camera{Zoom: 1}, last: map[entity.EntityID]pawnVisual{}}
}

// hotScenes is how many scenes keep their bakes: the viewed map and the
// one seen before it, so flipping between two maps never rebakes.
const hotScenes = 2

// sceneSet is every open map's scene and which one the Renderer shows.
type sceneSet struct {
	cur  game.MapID
	byID map[game.MapID]*mapScene
	// order is the scenes most recently shown first (order[0] is cur).
	order []game.MapID
	// resume is a loaded save's camera for each map not placed yet
	// (VIEW-001): applied when the scene is made, kept for the next save
	// until its camera is placed.
	resume map[game.MapID]camerapkg.View
	// tr is the shell's catalog for the overlays' words (nil: English).
	tr lang.Tr
}

// start makes id's scene the Renderer's (New, tests).
func (s *sceneSet) start(r *Renderer, id game.MapID) {
	s.cur = id
	s.order = append(s.order[:0], id)
	r.mapScene = s.ensure(id)
}

// ensure is id's scene, made on first use.
func (s *sceneSet) ensure(id game.MapID) *mapScene {
	if sc, ok := s.byID[id]; ok {
		return sc
	}
	if s.byID == nil {
		s.byID = map[game.MapID]*mapScene{}
	}
	sc := newMapScene()
	if v, ok := s.resume[id]; ok {
		sc.cam.Resume(v)
	}
	s.byID[id] = sc
	return sc
}

// follow switches to the viewed map's scene when the simulation's view
// moved (sim.ViewMap from ports.SwitchMap); once per Update and Draw.
func (s *sceneSet) follow(r *Renderer) {
	if r.game == nil || r.mapScene == nil {
		return
	}
	if id := r.game.ViewedMap(); id != s.cur {
		s.switchTo(r, id)
	}
}

// switchTo parks the shown scene and shows id's: the new scene inherits
// the player's overlay and detail choices, starts with no motion,
// particles or floaters, and hears the map as it is now, so nothing slides
// in from where it stood and no stale sound cue fires. Its bakes are kept;
// scenes past the hottest two free theirs, and closed maps are dropped.
func (s *sceneSet) switchTo(r *Renderer, id game.MapID) {
	old := r.mapScene
	s.park(old)
	sc := s.ensure(id)
	if old != nil && old.cam.Unit > 0 {
		// the window's cell (UIS-080) before the next clamp, so a jump
		// right after the switch (a colonist's portrait) centres in it
		sc.cam.SyncUnit(old.cam.Unit*camerapkg.CellsTall, nil)
	}
	s.cur = id
	r.mapScene = sc
	r.smooth.Halt() // no pan or zoom coasting onto the other map
	// the light grid is keyed on the last view's layers; start it over
	r.lit.Grid.Built = nil
	r.lit.Grid.Gen++
	p := r.pipe()
	carryChoices(old, sc, p)
	enterScene(r, sc)
	s.order = touchOrder(s.order, id)
	maps := r.game.Maps()
	open := make([]game.MapID, len(maps))
	for i, m := range maps {
		open[i] = m.ID
	}
	s.prune(open)
	for _, cold := range s.order[min(hotScenes, len(s.order)):] {
		s.chill(s.byID[cold])
	}
}

// park leaves a scene: its creatures' motion, particles and floaters are
// dropped (they would replay or slide when the map is shown again); the
// camera and the bakes stay.
func (s *sceneSet) park(sc *mapScene) {
	if sc == nil {
		return
	}
	sc.mo = motionpkg.State[creaturepkg.Body]{}
	clear(sc.last)
	sc.parts = sc.parts[:0]
	sc.floaters = sc.floaters[:0]
}

// chill frees a cold scene's bakes: the pipeline with its chunk and
// whole-map bakes (the map pages at once, the chunk images with the
// pipeline) and the snow cache. The camera, minimap and memories stay;
// the pipeline is rebuilt when the map is shown again.
func (s *sceneSet) chill(sc *mapScene) {
	if sc == nil {
		return
	}
	if sc.wp != nil {
		for _, pg := range sc.wp.LOD.Pages.Pages {
			if pg != nil {
				pg.Deallocate()
			}
		}
		sc.wp = nil
	}
	sc.snowc = envpkg.SnowCache{}
}

// prune drops the scenes of maps no longer open (the shown one stays).
func (s *sceneSet) prune(open []game.MapID) {
	isOpen := func(id game.MapID) bool {
		for _, o := range open {
			if o == id {
				return true
			}
		}
		return false
	}
	for id, sc := range s.byID {
		if id == s.cur || isOpen(id) {
			continue
		}
		s.chill(sc)
		delete(s.byID, id)
		delete(s.resume, id)
	}
	kept := s.order[:0]
	for _, id := range s.order {
		if _, ok := s.byID[id]; ok {
			kept = append(kept, id)
		}
	}
	s.order = kept
}

// views is every map's camera for the save sidecar (VIEW-001): each placed
// camera's view through v, and a loaded view still waiting for its map.
func (s *sceneSet) views(v camerapkg.Viewport) map[game.MapID]camerapkg.View {
	out := make(map[game.MapID]camerapkg.View, len(s.byID)+len(s.resume))
	for id, rv := range s.resume {
		out[id] = rv
	}
	for id, sc := range s.byID {
		if sc.cam.Placed {
			out[id] = sc.cam.View(v)
			delete(s.resume, id)
		}
	}
	return out
}

// resumeViews queues a loaded save's cameras: a scene that exists places
// its camera there on the next clamp; another map's waits for its scene.
func (s *sceneSet) resumeViews(views map[game.MapID]camerapkg.View) {
	for id, v := range views {
		if !v.Valid() {
			continue
		}
		if s.resume == nil {
			s.resume = map[game.MapID]camerapkg.View{}
		}
		s.resume[id] = v
		if sc, ok := s.byID[id]; ok {
			sc.cam.Resume(v)
		}
	}
}

// touchOrder moves id to the front of order.
func touchOrder(order []game.MapID, id game.MapID) []game.MapID {
	for i, o := range order {
		if o == id {
			copy(order[1:i+1], order[:i])
			order[0] = id
			return order
		}
	}
	order = append(order, 0)
	copy(order[1:], order)
	order[0] = id
	return order
}

// carryChoices copies the player's choices that live in a scene's layers
// from the scene left to the one entered (its pipeline p): the overlay
// toggles, the zone display, the colonist shadows, the grass detail and
// the minimap's visibility and anchor.
func carryChoices(old, sc *mapScene, p *world.Pipeline) {
	if old == nil || old == sc {
		return
	}
	sc.env.Shadow.SetOff(old.env.Shadow.Off)
	sc.mini.Off, sc.mini.GlobalH = old.mini.Off, old.mini.GlobalH
	if old.wp == nil {
		return
	}
	o, n := old.wp.Overlays, p.Overlays
	n.Roof, n.Home, n.Plan, n.Power, n.SnowClear = o.Roof, o.Home, o.Plan, o.Power, o.SnowClear
	n.Fertility, n.Temp, n.DeepOre = o.Fertility, o.Temp, o.DeepOre
	n.Combat.Cover = o.Combat.Cover
	n.TempDraw.Units = o.TempDraw.Units
	n.Zones.Hide, n.Zones.ShowAll = o.Zones.Hide, o.Zones.ShowAll
	p.LOD.Grass.Level = old.wp.LOD.Grass.Level
}

// enterScene readies the shown scene sc for the map as it is now: every
// item already lying there counts as old (no drop-in), the sound memories
// are primed with this world and the cues that priming raised are
// discarded (doors, deaths, blasts and pods that happened while the map
// was parked stay silent), and the pending thunder of the map left is
// cancelled. The particles priming emitted are dropped too.
func enterScene(r *Renderer, sc *mapScene) {
	w := r.game.World()
	if sc.itemBorn == nil {
		sc.itemBorn = map[entity.EntityID]uint64{}
	}
	clear(sc.itemBorn)
	for i := range w.Items {
		sc.itemBorn[w.Items[i].ID] = 0
	}
	cb := r.cueBus()
	n := len(cb.Cues)
	cb.ThunderAt = 0
	v, pawns := r.liveView(), r.game.Snapshot()
	sc.wp.Creatures.EmitWorldBursts(v, w)
	cb.ListenBursts(w.Bursts)
	set := &sc.wp.Overlays.Combat
	set.Shields = shieldStates(set.Shields[:0], pawns)
	cb.ListenShields(set.Shields)
	cb.ListenAnimals(v, w, 0)
	cb.ListenWorld(v, w, pawns)
	cb.Cues = cb.Cues[:n]
	sc.parts = sc.parts[:0]
	sc.heard = w
}
