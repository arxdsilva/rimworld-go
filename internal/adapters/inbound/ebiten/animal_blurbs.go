package ebiten

// animalBlurbs is the one-line description the animal inspector shows for
// each roster species, keyed by species key (internal/domain/animal).
// Original wording, at most 70 characters each.
var animalBlurbs = map[string]string{
	"deer":              "Skittish grazer. Bolts from people. Stags carry antlers.",
	"timber_wolf":       "Pack hunter of the woods. Tracks prey and bites hard.",
	"alpaca":            "Gentle woolly herd animal. Kicks when cornered.",
	"bison":             "Hump-shouldered brute. Gores anything that crowds its herd.",
	"emberbuck":         "Glowing glands on its back. Bursts into flame when killed.",
	"capybara":          "Calm, stocky river rodent. Content in a crowd.",
	"caribou":           "Antlered herd walker of the cold. Grazes in long lines.",
	"cow":               "Placid pen animal. Slow, sturdy, and a steady meat source.",
	"donkey":            "Stubborn, hardy beast. Tolerates heat and rough work.",
	"dromedary":         "One-humped desert walker. Tame it to carry on dry plains.",
	"elephant":          "Towering tusker. Guards its herd with crushing force.",
	"elk":               "Heavy deer of the north. Bulls swing wide antlers.",
	"gazelle":           "Quick desert grazer. Outruns nearly every hunter.",
	"goat":              "Hardy climber with a stubborn head. Lives anywhere.",
	"horse":             "Fast, strong runner. Easy to keep once tamed.",
	"ibex":              "Mountain grazer with swept-back horns. Rams intruders.",
	"ground_sloth":      "Shaggy giant with huge claws. Slow, but deadly up close.",
	"woolhorn":          "Shaggy, curl-horned grazer. Docile and thick-coated.",
	"pig":               "Round, slow forager. Bites if crossed.",
	"rhinoceros":        "Armored charger with a nose horn. Attacks without warning.",
	"sheep":             "Small woolly flock animal. Easy to tame and pen.",
	"yak":               "Long-haired mountain ox. Shrugs off the deepest cold.",
	"timbergnaw":        "Tree-gnawing builder with orange teeth. Bites when cornered.",
	"arctic_fox":        "White snow hunter. Stalks only small prey.",
	"arctic_wolf":       "Pale pack predator of the ice. Hunts in coordinated groups.",
	"cinder_rat":        "Ember-throated rodent. Detonates in a burst when killed.",
	"cat":               "Quiet pet and mouser. Scratches when scared.",
	"chinchilla":        "Soft grey puffball. Shy and always in a hurry.",
	"cougar":            "Lone tawny ambusher. Strikes fast with its claws.",
	"fennec_fox":        "Big-eared desert scavenger. Hunts only the smallest prey.",
	"grizzly_bear":      "Heavy brown hunter. Hits like a falling tree.",
	"guinea_pig":        "Tiny, round, and harmless. Breeds well in pens.",
	"hare":              "Long-eared sprinter. Vanishes at the first sign of danger.",
	"husky":             "Loyal sled dog with a thick coat. Guards its owner.",
	"labrador":          "Friendly gold retriever. Devoted to its handler.",
	"lynx":              "Tufted cold-country stalker. Pounces from cover.",
	"monkey":            "Quick, curious climber. Travels in noisy troops.",
	"panther":           "Black jungle hunter. Silent, fast, and dangerous.",
	"polar_bear":        "White giant of the ice. Mauls with huge paws.",
	"raccoon":           "Masked night raider. Runs, then scratches if caught.",
	"rat":               "Small, quick pest. Bites if trapped.",
	"red_fox":           "Orange-coated trickster. Hunts small animals only.",
	"snowhare":          "White winter hare. Blends into snow and bolts.",
	"squirrel":          "Tail-flicking tree runner. Gone before you arrive.",
	"dire_wolf":         "Massive-jawed, long-extinct hunter. Packs bring down giants.",
	"wild_boar":         "Bristled tusker. Charges anyone who gets close.",
	"yorkshire_terrier": "Tiny silky-coated lapdog. Barks far above its weight.",
	"cassowary":         "Blue-necked bird with a deadly kick. Fiercely territorial.",
	"chicken":           "Barnyard bird. Hens peck about; roosters strut and crow.",
	"duck":              "Flat-billed pond bird. Waddles, paddles, and pecks.",
	"emu":               "Tall shaggy runner. Flees fast, kicks when cornered.",
	"goose":             "Loud white guard bird. Hisses and pecks at strangers.",
	"ostrich":           "Giant flightless bird. Outsprints trouble, kicks to kill.",
	"turkey":            "Fan-tailed farm bird. Red wattle, ready peck.",
	"cobra":             "Hooded snake of dry lands. Strikes when startled.",
	"iguana":            "Sun-basking lizard. Slow to anger, quick to flee.",
	"tortoise":          "Armored and unhurried. Bites only if prodded.",
	"hive_beetle":       "Swarm insect with sharp mandibles. Never tameable.",
	"hive_brute":        "Hulking hive guardian. Tears through defenders.",
	"hive_crawler":      "Fast, many-legged hive scout. Overwhelms in numbers.",
	"glimmerstag":       "Legendary shining stag. Harm it and its herd will answer.",
}

// genericBlurb describes a species with no entry above.
const genericBlurb = "A wild creature. Watch how it behaves before you act."

// animalBlurb is the inspector's one-line description of a species key.
func animalBlurb(species string) string {
	if b, ok := animalBlurbs[species]; ok {
		return b
	}
	return genericBlurb
}
