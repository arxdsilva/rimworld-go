package ebiten

import (
	"math"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// Animal voices (SND-003): idle calls while wandering, a cry when fleeing,
// attack and hurt sounds from combat traces, a death cry, and the burst
// of explosive animals. Ids are animal.<key>.<event> (event: call,
// attack, hurt, die), falling back to animal.<group>.<event> and then
// animal.generic.<event>; presentation only.

// Animal sound events.
const (
	voiceCall   = "call"
	voiceAttack = "attack"
	voiceHurt   = "hurt"
	voiceDie    = "die"
	voiceBurst  = "animal.burst"
)

// voiceGroups are the fallback voices by kind of animal.
var voiceGroups = map[string]string{
	"deer": "large_mammal", "timber_wolf": "canid", "alpaca": "large_mammal", "bison": "bovid",
	"emberbuck": "large_mammal", "capybara": "small_mammal", "caribou": "large_mammal", "cow": "bovid",
	"donkey": "large_mammal", "dromedary": "large_mammal", "elephant": "large_mammal", "elk": "large_mammal",
	"gazelle": "large_mammal", "goat": "bovid", "horse": "large_mammal", "ibex": "bovid",
	"ground_sloth": "large_mammal", "woolhorn": "bovid", "pig": "large_mammal", "rhinoceros": "large_mammal",
	"sheep": "bovid", "yak": "bovid", "timbergnaw": "small_mammal", "arctic_fox": "canid",
	"arctic_wolf": "canid", "cinder_rat": "small_mammal", "cat": "felid", "chinchilla": "small_mammal",
	"cougar": "felid", "fennec_fox": "canid", "grizzly_bear": "large_mammal", "guinea_pig": "small_mammal",
	"hare": "small_mammal", "husky": "canid", "labrador": "canid", "lynx": "felid",
	"monkey": "small_mammal", "panther": "felid", "polar_bear": "large_mammal", "raccoon": "small_mammal",
	"rat": "small_mammal", "red_fox": "canid", "snowhare": "small_mammal", "squirrel": "small_mammal",
	"dire_wolf": "canid", "wild_boar": "large_mammal", "yorkshire_terrier": "canid", "cassowary": "bird_large",
	"chicken": "bird_small", "duck": "bird_small", "emu": "bird_large", "goose": "bird_small",
	"ostrich": "bird_large", "turkey": "bird_small", "cobra": "reptile", "iguana": "reptile",
	"tortoise": "reptile", "hive_beetle": "insect", "hive_brute": "insect", "hive_crawler": "insect",
	"glimmerstag": "large_mammal",
}

// voiceGroup is key's fallback voice group ("generic" when unlisted).
func voiceGroup(key string) string {
	if g, ok := voiceGroups[key]; ok {
		return g
	}
	return "generic"
}

// Timing, in frames (the call timers run on the game-speed clock; the
// rate caps on real frames).
const (
	firstCallMin   = 600  // a first call comes at firstCallMin + hash % firstCallSpread
	firstCallSpan  = 3000 //
	callEveryMin   = 1500 // then every callEveryMin + hash % callEverySpan
	callEverySpan  = 2700 //
	deferMin       = 120  // a call over the caps waits deferMin + hash % deferSpan
	deferSpan      = 181  //
	capCallsWindow = 60   // at most capCalls calls per capCallsWindow frames
	capCalls       = 2
	capSpeciesSpan = 600 // at most capSpecies distinct species per capSpeciesSpan frames
	capSpecies     = 3
	// largeSize is the AnimalView.Size at or above which the interval
	// stretches: the L class's body length (2.2, PLAY-022 C1) less its
	// -15% trim, so every L and XL animal counts (it was the old L 1.4).
	largeSize = 2.2 * 0.85
)

// Volumes of each event (the director adds distance and zoom).
const (
	volCall   = 0.7
	volAttack = 0.85
	volHurt   = 0.85
	volDie    = 1.0
	volBurst  = 1.0
	// volMeleeUnder is sfx.melee_hit under an animal's own voice.
	volMeleeUnder = 0.6
)

// petSpecies are the tame-able companions whose calls come more often.
var petSpecies = map[string]bool{"cat": true, "husky": true, "labrador": true, "yorkshire_terrier": true}

// voiceSeen is what an animal looked like last frame.
type voiceSeen struct {
	key  string
	x, y float64 // tile
}

// voicePlayed is one call that played, for the rate caps.
type voicePlayed struct {
	frame uint64
	key   string
}

// animalVoice is the renderer's memory of what each animal last did.
type animalVoice struct {
	// view is the colony viewport the App sets each frame; unset (zero)
	// means "on screen" and leaves culling to the director.
	view viewport
	// frame counts listenAnimals calls (real frames); clock is the game
	// speed clock the call timers run on (frozen while paused).
	frame, clock uint64
	nextCall     map[entity.EntityID]uint64
	calls        map[entity.EntityID]uint64 // calls scheduled so far, per animal
	fleeing      map[entity.EntityID]bool
	last         map[entity.EntityID]voiceSeen
	recent       []voicePlayed
	resolved     map[string]string          // key.event -> sound id ("" none)
	byID         map[entity.EntityID]string // this frame's animals: species key
}

func (v *animalVoice) init() {
	if v.nextCall == nil {
		v.nextCall = map[entity.EntityID]uint64{}
		v.calls = map[entity.EntityID]uint64{}
		v.fleeing = map[entity.EntityID]bool{}
		v.last = map[entity.EntityID]voiceSeen{}
		v.resolved = map[string]string{}
		v.byID = map[entity.EntityID]string{}
	}
}

// callInterval is how long animal an waits between calls (the first wait
// when first is set), scaled by the kind of animal.
func callInterval(an game.AnimalView, n uint64, first bool) uint64 {
	h := mix64(uint64(an.ID)*0x9E3779B97F4A7C15 ^ n*0xC2B2AE3D27D4EB4F)
	base := float64(callEveryMin + h%callEverySpan)
	if first {
		base = float64(firstCallMin + h%firstCallSpan)
	}
	group := voiceGroup(an.Species)
	scale := 1.0
	if petSpecies[an.Species] || strings.HasPrefix(group, "bird_") {
		scale *= 0.7
	}
	if an.Size >= largeSize {
		scale *= 1.3
	}
	if group == "insect" {
		scale *= 0.5
	}
	return uint64(base * scale)
}

// audible reports whether a world point (tiles) is within earshot.
func (r *Renderer) audible(x, y float64) bool {
	v := r.voice.view
	if v.W() <= 0 || v.H() <= 0 {
		return true
	}
	_, _, ok := spatial(r, x, y, v)
	return ok
}

// capsAllow reports whether another call by species key fits under the
// rate caps, now.
func (v *animalVoice) capsAllow(key string) bool {
	kept := v.recent[:0]
	for _, c := range v.recent {
		if v.frame-c.frame < capSpeciesSpan {
			kept = append(kept, c)
		}
	}
	v.recent = kept
	species := map[string]bool{key: true}
	inWindow := 0
	for _, c := range v.recent {
		species[c.key] = true
		if v.frame-c.frame < capCallsWindow {
			inWindow++
		}
	}
	return inWindow < capCalls && len(species) <= capSpecies
}

// listenAnimals cues calls, flee cries and death cries from this frame's
// animals (called from advance). Idle calls run on the game-speed clock
// (none while paused), only for animals within earshot, and under the
// caps of capsAllow; a call over a cap is deferred, never dropped.
func (r *Renderer) listenAnimals(w game.WorldView) {
	v := &r.voice
	v.init()
	v.frame++
	v.clock += uint64(r.mo.Stepped) // ticks this advance ran (the Update budget may run fewer)
	clear(v.byID)
	for i := range w.Animals {
		v.byID[w.Animals[i].ID] = w.Animals[i].Species
	}
	// deaths: ids that were here last frame and are gone, with a corpse
	for id, was := range v.last {
		if _, here := v.byID[id]; here {
			continue
		}
		if corpseNear(w.Items, was) {
			r.cueAnimal(was.key, voiceDie, was.x+0.5, was.y+0.5, volDie)
		}
		delete(v.nextCall, id)
		delete(v.calls, id)
		delete(v.fleeing, id)
		delete(v.last, id)
	}
	for i := range w.Animals {
		an := &w.Animals[i]
		id := an.ID
		var x, y float64
		placed := false
		at := func() (float64, float64) { // where it is drawn, sampled only for a cue
			if !placed {
				x, y = r.interpAnimal(*an)
				x, y, placed = x+0.5, y+0.5, true
			}
			return x, y
		}
		_, seen := v.last[id]
		v.last[id] = voiceSeen{key: an.Species, x: float64(an.X), y: float64(an.Y)}
		if an.Mode == "fleeing" {
			cry := seen && !v.fleeing[id] // an animal first seen already fleeing stays quiet
			v.fleeing[id] = true
			if cry {
				if x, y := at(); r.audible(x, y) && r.cueAnimal(an.Species, voiceCall, x, y, volCall) {
					v.recent = append(v.recent, voicePlayed{v.frame, an.Species})
				}
				v.calls[id]++
				v.nextCall[id] = v.clock + callInterval(*an, v.calls[id], false)
			}
			continue
		}
		v.fleeing[id] = false
		next, ok := v.nextCall[id]
		if !ok {
			v.nextCall[id] = v.clock + callInterval(*an, 0, true)
			continue
		}
		if v.clock < next {
			continue
		}
		v.calls[id]++
		n := v.calls[id]
		switch {
		case an.Mode != "wandering" || !r.audible(at()):
			v.nextCall[id] = v.clock + callInterval(*an, n, false)
		case !v.capsAllow(an.Species):
			v.nextCall[id] = v.clock + deferMin + mix64(uint64(id)^n<<32)%deferSpan
		default:
			if x, y := at(); r.cueAnimal(an.Species, voiceCall, x, y, volCall) {
				v.recent = append(v.recent, voicePlayed{v.frame, an.Species})
			}
			v.nextCall[id] = v.clock + callInterval(*an, n, false)
		}
	}
}

// corpseNear reports whether a corpse of was's species lies within a tile
// of where it was last seen.
func corpseNear(items []game.ItemView, was voiceSeen) bool {
	for _, it := range items {
		if it.Animal == was.key && !it.Carried &&
			math.Abs(float64(it.X)-was.x) <= 1 && math.Abs(float64(it.Y)-was.y) <= 1 {
			return true
		}
	}
	return false
}

// cueAnimal cues key's sound for event at a point; it reports whether
// there was a sound to cue.
func (r *Renderer) cueAnimal(key, event string, x, y, vol float64) bool {
	id := r.animalSoundID(key, event)
	if id == "" {
		return false
	}
	r.cue(id, x, y, vol)
	return true
}

// animalSoundID resolves key's sound for event through the fallback chain
// (species, voice group, generic; "" when nothing exists). A nil soundHas
// means every sound exists. The answer is cached.
func (r *Renderer) animalSoundID(key, event string) string {
	v := &r.voice
	v.init()
	ck := key + "." + event
	if id, ok := v.resolved[ck]; ok {
		return id
	}
	id := ""
	for _, c := range []string{
		"animal." + key + "." + event,
		"animal." + voiceGroup(key) + "." + event,
		"animal.generic." + event,
	} {
		if r.soundHas == nil || r.soundHas(c) {
			id = c
			break
		}
	}
	v.resolved[ck] = id
	return id
}
