package cover

// Kind is what stands on a tile as far as shooting is concerned.
type Kind uint8

const (
	None Kind = iota
	Sandbags
	Barricade
	Wall
	Tree
	Rock  // natural rock
	Chunk // rock chunk
	// DEF-3 (append only): doors, plants and furniture beside a target,
	// and the turrets (a light and a heavy one) a raider crouches behind.
	Door
	Cactus
	Fence
	Bush
	Stool // a stool or a plant pot
	Table // a table or a bed
	Chair // a dining chair
	Turret
	HeavyTurret
	numKinds
)

// block is the permille of shots a kind absorbs. wiki: Cover
var block = [numKinds]int{
	None:      0,
	Sandbags:  550, // wiki: Cover
	Barricade: 550, // wiki: Cover (57 before 1.3)
	Wall:      750, // wiki: Cover
	Tree:      250, // wiki: Cover
	// wiki conflict: no page value for natural rock; it is a wall.
	Rock:  750,
	Chunk: 500, // wiki: Cover (stone or slag chunk)
	Door:  750, // wiki: Cover
	// wiki: Cover (saguaro cactus, fence, bush, plant pot or stool,
	// table or bed, dining chair)
	Cactus: 350,
	Fence:  250,
	Bush:   200,
	Stool:  200,
	Table:  400,
	Chair:  350,
	// wiki: Mini-turret, Autocannon turret, Uranium slug turret (cover
	// effectiveness)
	Turret:      400,
	HeavyTurret: 500,
}

// Permille returns what share of shots k absorbs (0 for unknown kinds).
func Permille(k Kind) int {
	if k >= numKinds {
		return 0
	}
	return block[k]
}

// Grid is the consumer-owned view of the map.
type Grid interface {
	// CoverAt is the cover kind on tile (x, y); None off-map.
	CoverAt(x, y int) Kind
}

// Best picks the strongest cover among the up to three tiles adjacent to
// the target that lie toward the shooter (horizontal, vertical and
// diagonal neighbour). Ties keep the first found in that order. Same
// tile or no cover returns (None, 0).
func Best(g Grid, shooterX, shooterY, targetX, targetY int) (Kind, int) {
	k, p, _, _ := BestAt(g, shooterX, shooterY, targetX, targetY)
	return k, p
}

// BestAt is Best with the tile the cover stands on (the target's own
// tile when there is none).
func BestAt(g Grid, shooterX, shooterY, targetX, targetY int) (best Kind, bp, x, y int) {
	dx, dy := sign(shooterX-targetX), sign(shooterY-targetY)
	var cand [3][2]int
	n := 0
	if dx != 0 {
		cand[n] = [2]int{targetX + dx, targetY}
		n++
	}
	if dy != 0 {
		cand[n] = [2]int{targetX, targetY + dy}
		n++
	}
	if dx != 0 && dy != 0 {
		cand[n] = [2]int{targetX + dx, targetY + dy}
		n++
	}
	x, y = targetX, targetY
	for _, c := range cand[:n] {
		k := g.CoverAt(c[0], c[1])
		if p := Permille(k); p > bp {
			best, bp, x, y = k, p, c[0], c[1]
		}
	}
	return best, bp, x, y
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}
