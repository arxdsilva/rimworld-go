package scenario

import (
	"fmt"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/pawn/prisoner"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
)

// SocialSide is the smallest map side the social colony fits.
const SocialSide = 60

// SocialCast names the people, animal and beds the social colony places,
// so tests and captures can find them.
type SocialCast struct {
	Couple             [2]entity.EntityID // share the double bed
	Siblings           [2]entity.EntityID // kin; their parents are seeded off the map
	Engaged            [2]entity.EntityID // seated at the marriage spot
	Rival              [2]entity.EntityID // placed side by side in the workshop
	Prisoner, Warden   entity.EntityID
	Dog                entity.EntityID
	Scarred            entity.EntityID // a jaw scar: disfigured
	DoubleBed, PrisBed entity.EntityID
}

// Social lays out the small walled colony used by -scenario social and the
// social captures, on a fresh seeded w x h map (at least SocialSide). Only
// public dev hooks and typed commands are used. The relationship graph is
// whatever the simulation seeds (off-map parents and siblings) plus what
// the dev hooks stage: Maren and Tobias are married, Eli and Fen engaged
// (their wedding happens at the marriage spot), Hale and Iris rivals,
// Biscuit bonded to Maren and Cyrus has a scarred jaw.
//
//   - a walled compound, 29 x 17, with a south door; an inner wall splits
//     off a prisoner cell east, with its own door;
//   - two bedrooms' worth of beds: a double bed for a couple, sleeping
//     spots for the rest, a prisoner bed in the cell;
//   - a marriage spot with chairs and a table, a brewing bench with a
//     brew-beer bill, grain and beer on the floor, a feeding trough and food;
//   - a colonist arrested by the warden (on Recruit) and a warden;
//   - a tame husky bonded to, and mastered by, one of the couple.
func Social(seed uint64, w, h int) (*game.Game, SocialCast, error) {
	var c SocialCast
	if w < SocialSide || h < SocialSide {
		return nil, c, fmt.Errorf("social scenario needs a map of at least %dx%d, got %dx%d", SocialSide, SocialSide, w, h)
	}
	g := game.NewColony(game.Config{Seed: seed, Width: w, Height: h})
	cx, cy := w/2, h/2
	g.ClearPlants(cx-16, cy-10, cx+16, cy+10)
	x0, x1, y0, y1 := cx-14, cx+14, cy-8, cy+8
	partition := cx + 7
	for x := x0; x <= x1; x++ {
		for _, y := range []int{y0, y1} {
			kind := "wall"
			if y == y1 && x == cx-3 {
				kind = "door"
			}
			g.BuildNow(kind, wallStuff(kind), "south", x, y)
		}
	}
	for y := y0 + 1; y < y1; y++ {
		for _, x := range []int{x0, x1, partition} {
			kind := "wall"
			if x == partition && y == cy+3 {
				kind = "door"
			}
			g.BuildNow(kind, wallStuff(kind), "south", x, y)
		}
	}
	// beds: the couple's double bed, sleeping spots, the cell's prisoner bed
	g.BuildNow("double bed", "wood", "south", cx-12, y0+2)
	for i := range 4 {
		g.BuildNow("sleeping spot", "wood", "south", cx-8+2*i, y0+1)
	}
	g.BuildNow("sleeping spot", "wood", "south", partition+3, y0+2)
	// the wedding corner: a marriage spot between chairs, a table, a lamp
	g.BuildNow("marriage spot", "wood", "south", cx-6, cy+3)
	for _, p := range [][2]int{{cx - 7, cy + 3}, {cx - 4, cy + 3}, {cx - 8, cy}, {cx - 5, cy}} {
		g.BuildNow("chair", "wood", "south", p[0], p[1])
	}
	g.BuildNow("table", "wood", "south", cx-7, cy)
	g.BuildNow("lamp", "wood", "south", cx-9, cy+5)
	// the workshop: a brewing bench with a bill, grain and beer
	g.BuildNow("brewing bench", "wood", "south", cx+2, y0+2)
	_, _ = g.SpawnStack(item.Brewgrain, 24, cx+2, y0+4)
	_, _ = g.SpawnStack(item.Beer, 6, cx+4, y0+4)
	g.Enqueue(command.Command{Kind: command.AddBill, X: cx + 2, Y: y0 + 2, Recipe: uint8(recipe.BrewBeer), Mode: uint8(recipe.DoCount), Count: 6})
	// food: a trough and meat for the dog, meals and berries for the people
	g.BuildNow("feeding trough", "wood", "south", cx-10, cy+6)
	_, _ = g.SpawnStack(item.Meat, 60, cx-11, cy+6)
	_, _ = g.SpawnStack(item.Meal, 20, cx-4, cy+6)
	_, _ = g.SpawnStack(item.Berries, 40, cx-2, cy+6)
	g.Step(1) // let the blueprints settle so the ids below exist

	for _, b := range g.World().Buildings {
		switch {
		case b.Kind == "double bed":
			c.DoubleBed = b.ID
		case b.Kind == "sleeping spot" && b.X == partition+3:
			c.PrisBed = b.ID
		}
	}

	// people: the start kit keeps its names; the rest join them by name
	spawn := func(name string, x, y int) entity.EntityID {
		id, err := g.SpawnNamedPawn(x, y, name)
		if err != nil {
			panic(fmt.Sprintf("social scenario: %v", err))
		}
		return id
	}
	c.Couple = [2]entity.EntityID{spawn("Maren", cx-12, y0+3), spawn("Tobias", cx-11, y0+3)}
	c.Siblings = [2]entity.EntityID{spawn("Cyrus", cx-3, y0+3), spawn("Dee", cx-2, y0+3)}
	c.Engaged = [2]entity.EntityID{spawn("Eli", cx-7, cy+1), spawn("Fen", cx-5, cy+1)}
	c.Rival = [2]entity.EntityID{spawn("Hale", cx+1, y0+5), spawn("Iris", cx+3, y0+5)}
	c.Warden = spawn("Wren", cx+5, cy+2)
	c.Prisoner = spawn("Garr", cx+3, cy+5)

	g.Enqueue(command.Command{Kind: command.AssignBedOwner, Target: uint64(c.DoubleBed), Index: 0, Pawn: uint64(c.Couple[0])})
	g.Enqueue(command.Command{Kind: command.AssignBedOwner, Target: uint64(c.DoubleBed), Index: 1, Pawn: uint64(c.Couple[1])})
	g.Enqueue(command.Command{Kind: command.SetBedForPrisoners, Target: uint64(c.PrisBed), Enabled: true})
	g.Step(1)
	g.Enqueue(command.Command{Kind: command.Arrest, Pawn: uint64(c.Warden), Target: uint64(c.Prisoner)})
	g.Enqueue(command.Command{Kind: command.SetPrisonerMode, Target: uint64(c.Prisoner), Mode: uint8(prisoner.Recruit)})

	// a husky, tamed and mastered by one of the couple
	id, err := g.SpawnAnimal("husky", cx-10, y0+5)
	if err != nil {
		return nil, c, err
	}
	g.TameAnimal(id, "Biscuit")
	g.DevBond(id, c.Couple[0])
	g.Enqueue(command.Command{Kind: command.SetAnimalMaster, Pawn: uint64(c.Couple[0]), Target: uint64(id)})
	c.Dog = id

	// the staged relationships
	g.DevSetPair(c.Couple[0], c.Couple[1], "spouse")
	g.DevSetPair(c.Engaged[0], c.Engaged[1], "fiance")
	for range 3 {
		g.DevAddOpinionMemory(c.Rival[0], c.Rival[1], "insulted")
		g.DevAddOpinionMemory(c.Rival[1], c.Rival[0], "insulted")
	}
	c.Scarred = c.Siblings[0]
	g.DevScar(c.Scarred, "jaw")
	g.Step(1)
	return g, c, nil
}

func wallStuff(kind string) string {
	if kind == "door" {
		return "wood" // doors are wood or steel, never stone
	}
	return "granite blocks"
}
