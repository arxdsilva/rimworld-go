package scenario

import (
	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/research"
)

func startItem(k item.Kind, qty int32) Part {
	return Part{Kind: PartStartItem, Item: k, Qty: qty}
}

func scatter(k item.Kind, qty int32, piles uint8) Part {
	return Part{Kind: PartScatterItem, Item: k, Qty: qty, N: piles, Near: true}
}

func animalPart(s animal.Species, n uint8) Part {
	return Part{Kind: PartStartAnimal, Species: s, N: n}
}

func grant(p research.Project) Part { return Part{Kind: PartResearch, Project: p} }

func intro(text string) Part { return Part{Kind: PartIntro, Text: text} }

// Built-in seeds are fixed so a built-in scenario's own random parts
// (the random pet, forced-trait draws) are the same on every machine.
const (
	seedBrokenHull       = 0x1b0e_0001
	seedEmberClan        = 0x1b0e_0002
	seedGildedExpedition = 0x1b0e_0003
	seedBareHands        = 0x1b0e_0004
)

// Builtins are the four built-in scenarios in display order (NG-1 §3.1).
// Every call returns fresh copies the caller may edit.
func Builtins() []Def {
	return []Def{
		{
			Key: "broken_hull", Name: "Broken Hull", Builtin: true, Seed: seedBrokenHull,
			Summary: "Three crew members survive a hull breach and ride the lifeboats down.",
			Desc: "The freighter Marrow Light split along a weld nobody had inspected in years. " +
				"Three of her crew reached the lifeboats, along with a locker of supplies, a little salvage and a pet that refused to be left behind. " +
				"You have industrial tools, medicine and rifles, but no walls, no beds and no idea who else is down here. " +
				"A balanced start for learning how the colony works.",
			Faction: faction.Settlers, People: 3, ChooseFrom: 8, Arrival: DropPods, Tech: faction.Industrial, Hardness: 2,
			Parts: []Part{
				startItem(item.SurvivalMeal, 40),
				startItem(item.Meal, 10),
				startItem(item.IndustrialMedicine, 12),
				startItem(item.HerbalMedicine, 6),
				startItem(item.Components, 20),
				startItem(item.Steel, 300),
				startItem(item.Wood, 200),
				startItem(item.Silver, 600),
				startItem(item.Rifle, 2),
				startItem(item.Knife, 1),
				startItem(item.Parka, 1),
				startItem(item.Duster, 1),
				startItem(item.Tuque, 1),
				{Kind: PartStartAnimal, Random: true, N: 1},
				scatter(item.Steel, 400, 4),
				scatter(item.SurvivalMeal, 8, 1),
				scatter(item.Components, 6, 1),
				grant(research.FoodPreservation),
				grant(research.Traps),
				intro("The alarms are still ringing in your ears when the lifeboats strike the ground. " +
					"Hatches pop one after another, and the three of you tumble out into cold, clean air. " +
					"Behind you the sky is full of falling hull plates, some of them burning. " +
					"Somewhere in the wreckage is the locker you grabbed on the way out, and in the locker is everything you own. " +
					"There is no rescue beacon and no schedule. There is daylight, and a short while before the dark. " +
					"Count your people, gather what fell, and decide where home will be."),
			},
		},
		{
			Key: "ember_clan", Name: "Ember Clan", Builtin: true, Seed: seedEmberClan,
			Summary: "Five clanfolk carry their hearth-fire to new land.",
			Desc: "Five clanfolk walk out of the old valley with a banked coal in a clay pot and every story the elders could teach them. " +
				"They know fire, hide and bone, and they trust each other because there is nobody else. " +
				"Research is slow without books, and rifles are only a rumour. " +
				"Plant early, keep the hearth lit and keep the goats close.",
			Faction: faction.Clans, People: 5, ChooseFrom: 10, Arrival: Standing, Tech: faction.Neolithic, Hardness: 3,
			Parts: []Part{
				startItem(item.Meat, 40),
				startItem(item.Berries, 60),
				startItem(item.HerbalMedicine, 10),
				startItem(item.Wood, 120),
				startItem(item.Leather, 30),
				startItem(item.Spear, 3),
				startItem(item.Club, 2),
				startItem(item.Knife, 2),
				startItem(item.Tuque, 5),
				animalPart(animal.Goat, 1),
				animalPart(animal.Chicken, 1),
				intro("You carry the coal in a pot of clay wrapped in wet moss, and you have not let it out of your sight for eleven days. " +
					"The valley is behind you now: the old hearth, the elders' voices, the stones with your families' names scratched on them. " +
					"Ahead is land that nobody has named yet. " +
					"Find shelter before dusk, set the coal to kindling, and let the smoke rise. " +
					"Others may see it. Some will be glad. Not all of them."),
			},
		},
		{
			Key: "gilded_expedition", Name: "Gilded Expedition", Builtin: true, Seed: seedGildedExpedition,
			Summary: "One wealthy explorer, well armed and well supplied, alone.",
			Desc: "One person, one pod, a fortune in silver and the confidence to match. " +
				"You arrive well fed, well armed and well medicated, with a stack of research already in your head. " +
				"What you do not have is anyone to dig, haul, cook or keep watch. " +
				"Wealth draws attention, so the first seasons are about turning coin into companions before something notices the shine.",
			Faction: faction.Settlers, People: 1, ChooseFrom: 6, Arrival: DropPods, Tech: faction.Industrial, Hardness: 3,
			Parts: []Part{
				startItem(item.Meal, 20),
				startItem(item.SurvivalMeal, 20),
				startItem(item.IndustrialMedicine, 20),
				startItem(item.Components, 30),
				startItem(item.Steel, 400),
				startItem(item.Silver, 3000),
				startItem(item.Gold, 30),
				startItem(item.Rifle, 1),
				startItem(item.Knife, 1),
				startItem(item.Parka, 1),
				startItem(item.Duster, 1),
				grant(research.FoodPreservation),
				grant(research.Herbalism),
				grant(research.Tailoring),
				grant(research.Traps),
				grant(research.Gunsmithing),
				intro("The pod's door opens with a soft pneumatic sigh, and sunlight pours in across a crate of silver you have no way to spend. " +
					"For the first time in a long career, nobody is taking notes. " +
					"You came here to see something no one has mapped, and you were certain you could manage it alone. " +
					"The rifle at your hip is a comfort. The silence is less so. " +
					"Take stock, find water, and start thinking about who you could hire when there is no one to hire."),
			},
		},
		{
			Key: "bare_hands", Name: "Bare Hands", Builtin: true, Seed: seedBareHands,
			Summary: "One survivor, no clothes, no tools, no food.",
			Desc: "One survivor, tired to the bone, with nothing but skin and stubbornness. " +
				"No clothes, no tools, no food. " +
				"Everything you eat tomorrow is something you find or make today. " +
				"This is the hardest start and the quietest, and the person who walks out of the wreck is nobody you chose.",
			Faction: faction.Settlers, People: 1, ChooseFrom: 6, Arrival: DropPods, Tech: faction.Industrial, Hardness: 4,
			Parts: []Part{
				{Kind: PartNoClothes},
				{Kind: PartForcedCondition, PawnCond: pawngen.Exhausted, Chance: 100},
				intro("You wake face down in the dirt, ears ringing, with the taste of metal in your mouth. " +
					"Everything you owned burned up on the way down, and so did your last good idea about what to do next. " +
					"The wind is cold. The ground is hard. " +
					"Somewhere nearby there is water and something edible, and you are going to have to be the one to find both. " +
					"Stand up first. The rest can wait until you are standing."),
			},
		},
	}
}

// Builtin finds a built-in scenario by key.
func Builtin(key string) (Def, bool) {
	for _, d := range Builtins() {
		if d.Key == key {
			return d, true
		}
	}
	return Def{}, false
}
