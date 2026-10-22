package ebiten

import (
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

// animalVoice is the renderer's memory of what each animal last did.
// SND-003 ST: fill in (timers, flee memo, last seen, rate window, cache).
type animalVoice struct {
	nextCall map[entity.EntityID]uint64
}

// listenAnimals cues calls, flee cries and death cries from this frame's
// animals (called from advance). SND-003 ST: implement.
func (r *Renderer) listenAnimals(w game.WorldView) {}

// animalSoundID resolves key's sound for event through the fallback chain
// ("" when nothing exists). SND-003 ST: implement.
func (r *Renderer) animalSoundID(key, event string) string { return "" }
