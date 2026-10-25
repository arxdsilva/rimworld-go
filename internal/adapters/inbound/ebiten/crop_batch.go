package ebiten

// Crop units (PLAY-016, ART spec §9.3): crops are composed per tile from
// several small unit sprites, like grass. Stub: RENDER-CROPS fills the
// layout, jitter, mirroring, growth scale, sway, batching and LOD.

// cropUnitsPerTile is how many unit sprites a crop's tile shows.
var cropUnitsPerTile = map[string]int{
	"corn": 2, "wheat": 4, "brewgrain": 4, "cotton": 3, "potato": 3, "beans": 3,
	"healroot": 3, "ashleaf": 3, "lettuce": 4, "radish": 4, "pumpkin": 1,
}
