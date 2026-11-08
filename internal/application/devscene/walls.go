package devscene

import (
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
)

// wallScene builds the -scenario walls set piece around (cx, cy): wood
// and granite walls in every connection case, a five-stone rock strip, a
// mined-out hole, a wall run into an outcrop, rough beside smoothed rock,
// an ore vein, steel walls and a chunk by the rock.
func wallScene(g *game.Game, cx, cy int) {
	x0, y0 := cx-17, cy-8
	g.ClearPlants(x0-1, y0-1, cx+18, cy+7)
	g.SetGround(x0-1, y0-1, cx+18, cy+7, "soil", "", "")
	at := func(dx, dy int) (int, int) { return x0 + dx, y0 + dy }
	build := func(kind, stuff string, cells ...[2]int) {
		for _, c := range cells {
			x, y := at(c[0], c[1])
			g.BuildNow(kind, stuff, "south", x, y)
		}
	}
	ground := func(dx0, dy0, dx1, dy1 int, t, stone, ore string) {
		ax, ay := at(dx0, dy0)
		bx, by := at(dx1, dy1)
		g.SetGround(ax, ay, bx, by, t, stone, ore)
	}
	// row 1: wood shapes, a granite 2x2 and ring, diagonal-only walls
	build("wall", "wood", [2]int{0, 0})                                                              // isolated
	build("wall", "wood", [2]int{2, 0}, [2]int{3, 0}, [2]int{4, 0})                                  // straight
	build("wall", "wood", [2]int{6, 0}, [2]int{7, 0}, [2]int{6, 1})                                  // L
	build("wall", "wood", [2]int{9, 0}, [2]int{10, 0}, [2]int{11, 0}, [2]int{10, 1})                 // T
	build("wall", "wood", [2]int{14, 0}, [2]int{13, 1}, [2]int{14, 1}, [2]int{15, 1}, [2]int{14, 2}) // cross
	build("wall", "granite blocks", [2]int{17, 0}, [2]int{18, 0}, [2]int{17, 1}, [2]int{18, 1})      // 2x2
	for dx := 20; dx <= 22; dx++ {
		for dy := 0; dy <= 2; dy++ {
			if dx == 21 && dy == 1 {
				continue
			}
			kind := "wall"
			if dx == 21 && dy == 2 {
				kind = "door"
			}
			stuff := "granite blocks"
			if kind == "door" {
				stuff = "wood"
			}
			build(kind, stuff, [2]int{dx, dy})
		}
	}
	build("wall", "wood", [2]int{24, 0}, [2]int{25, 1}) // diagonal only
	// row 2: five stones side by side, a mined-out 3x3, a wall into rock,
	// rough beside smoothed rock over a smooth floor
	for i, st := range []string{"sandstone", "granite", "limestone", "slate", "marble"} {
		ground(i*3, 4, i*3+2, 6, "rock", st, "")
	}
	ground(16, 4, 18, 6, "rock", "granite", "")
	ground(17, 5, 17, 5, "stone", "granite", "")
	ground(20, 4, 22, 6, "rock", "granite", "")
	build("wall", "granite blocks", [2]int{23, 5}, [2]int{24, 5}, [2]int{25, 5})
	ground(27, 4, 29, 6, "rock", "marble", "")
	ground(30, 4, 32, 6, "rock_smooth", "marble", "")
	ground(27, 7, 32, 7, "stone_smooth", "marble", "")
	// row 3: ore veins in granite, a chunk beside it, steel walls and door
	ground(0, 10, 6, 13, "rock", "granite", "")
	ground(2, 11, 3, 12, "rock", "granite", "steel")
	ground(4, 12, 5, 12, "rock", "granite", "silver")
	ground(1, 13, 1, 13, "rock", "granite", "gold")
	if x, y := at(7, 11); true {
		_, _ = g.SpawnStack(item.GraniteChunk, 1, x, y)
	}
	build("wall", "steel", [2]int{10, 11}, [2]int{11, 11}, [2]int{13, 11}, [2]int{14, 11}, [2]int{14, 12})
	build("door", "steel", [2]int{12, 11})
}
