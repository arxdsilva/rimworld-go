package world

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"rimworld-go/internal/adapters/inbound/ebiten/render/overlay"
	"rimworld-go/internal/adapters/inbound/ebiten/render/sprite"
	"rimworld-go/internal/application/game"
)

// TestItemSpriteFallbackUsesUnderscores: kinds with spaces find their
// sprite (item names have spaces, sprite names underscores).
func TestItemSpriteFallbackUsesUnderscores(t *testing.T) {
	for kind, want := range map[string]string{
		"servo arm":   "item/servo_arm",
		"nanomed kit": "item/nanomed_kit",
		"peg leg":     "item/peg_leg",
		"kidney":      "item/kidney",
	} {
		if got := ItemSprite(kind); got != want {
			t.Errorf("ItemSprite(%q) = %q, want %q", kind, got, want)
		}
	}
}

func TestBuildSpriteStuffMaterials(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind, stuff string
		mask        int
		ew, open    bool
		want        string
	}{
		{"wall", "steel", 5, false, false, "structure/wall_steel_05"},
		{"wall", "slate blocks", 15, false, false, "structure/wall_stone_15"},
		{"wall", "stone", 3, false, false, "structure/wall_stone_03"},
		{"wall", "wood", 3, false, false, "structure/wall_wood_03"},
		{"door", "steel", 10, true, true, "structure/door_steel_ew_open"},
		{"door", "steel", 5, false, false, "structure/door_steel_ns_closed"},
		{"door", "wood", 5, false, false, "structure/door_wood_ns_closed"},
		{"door", "wood", 0, sprite.DoorAxisEW("south"), false, "structure/door_wood_ew_closed"}, // axis from the stored facing, not the links mask
		{"door", "steel", 15, sprite.DoorAxisEW("west"), true, "structure/door_steel_ns_open"},
		{"floor", "granite blocks", 0, false, false, "structure/floor_stone"},
		{"fence", "marble blocks", 15, false, false, "structure/fence_stone_15"},
	}
	for _, c := range cases {
		if got := BuildSpriteStuff(c.kind, c.stuff, "", c.mask, c.ew, c.open); got != c.want {
			t.Fatalf("BuildSpriteStuff(%s, %s) = %s, want %s", c.kind, c.stuff, got, c.want)
		}
	}
	if sprite.WallPrefix("granite blocks") != "structure/wall_stone" || sprite.WallPrefix("steel") != "structure/wall_steel" || sprite.WallPrefix("wood") != "structure/wall_wood" {
		t.Fatal("WallPrefix")
	}
}

func TestSteamVentResolvesToItsSprite(t *testing.T) {
	t.Parallel()
	if got := BuildSprite("steam vent", "", 0, false, false); got != "structure/steam_vent" {
		t.Fatalf("sprite = %q", got)
	}
	if got := BuildSpriteStuff("steam vent", "", "", 0, false, false); got != "structure/steam_vent" {
		t.Fatalf("stuff sprite = %q", got)
	}
}

// TestBuildSpriteKinds pins the building keys the world pass and the
// placement ghost draw: facings, gates, carpets, power parts, doors.
func TestBuildSpriteKinds(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		kind, stuff, facing string
		mask                int
		open                bool
		want                string
	}{
		{"bed", "", "", 0, false, "furniture/bed_basic_south"},
		{"double bed", "", "east", 0, false, "furniture/bed_double_east"},
		{"fence gate", "", "west", 0, true, "structure/fence_gate_open_west"},
		{"fence gate", "", "", 0, false, "structure/fence_gate_south"},
		{"carpet", "cloth carpet", "", 0, false, "furniture/carpet_cloth"},
		{"carpet", "silk", "", 0, false, "furniture/carpet_straw"},
		{"power conduit", "", "", 0x15, false, "power/power_conduit_05"},
		{"power switch", "", "", 0, true, "power/power_switch_off"},
		{"power switch", "", "", 0, false, "power/power_switch_on"},
		{"grave", "", "west", 0, false, "furniture/grave_dug_east"},
		{"no such kind", "", "", 0, false, ""},
	} {
		if got := BuildSpriteStuff(c.kind, c.stuff, c.facing, c.mask, false, c.open); got != c.want {
			t.Errorf("BuildSpriteStuff(%q, %q, %q, %d, open %v) = %q, want %q", c.kind, c.stuff, c.facing, c.mask, c.open, got, c.want)
		}
	}
	if GateSprite("north", false) != "structure/fence_gate_north" || FacingOr("") != "south" {
		t.Fatal("gate sprite or default facing")
	}
	if TroughKey(game.BuildingView{TroughFill: 3, Facing: "east"}) != "furniture/feeding_trough_full_east" ||
		TroughKey(game.BuildingView{}) != "furniture/feeding_trough_empty_south" {
		t.Fatal("trough keys")
	}
}

func TestTerrainEdgePriorityMatchesArt(t *testing.T) {
	t.Parallel()
	// the edge art's order: soil < soil_rich < gravel < stone < stone_smooth < sand < marsh < ice
	order := []string{"soil", "rich_soil", "gravel", "stone", "stone_smooth", "sand", "marsh", "ice"}
	for i, n := range order {
		if TerrainPriority[n] != i || TerrainByPri[i] != n {
			t.Fatalf("%s: priority %d, want %d", n, TerrainPriority[n], i)
		}
	}
	if TerrainPriority["rock"] != TerrainPriority["stone"] || TerrainPriority["rock_smooth"] != TerrainPriority["stone_smooth"] {
		t.Fatal("rock draws its ground as the matching stone")
	}
	if TerrainSprite["rock_smooth"] != "stone_smooth" || TerrainSprite["stone_smooth"] != "stone_smooth" {
		t.Fatal("smooth ground sprite")
	}
	// the per-code tables follow the name tables
	for c, n := range TerrainNames {
		if int(TerrainPriByCode[c]) != TerrainPriority[n] || IsWaterCode[c] != (n == "shallow_water" || n == "deep_water") {
			t.Fatalf("code %d (%s): priority %d, water %v", c, n, TerrainPriByCode[c], IsWaterCode[c])
		}
	}
	if TerrainEdgeKeys[TerrainPriority["sand"]][5] != "terrain/sand_edge_05" {
		t.Fatalf("edge key %q", TerrainEdgeKeys[TerrainPriority["sand"]][5])
	}
}

// TestSizedItemKey: components and industrial medicine pick their pile
// by quantity; any other key is kept.
func TestSizedItemKey(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		key  string
		qty  int
		want string
	}{
		{"item/components_m", 1, "item/components_s"},
		{"item/components_m", 14, "item/components_m"},
		{"item/components_m", 15, "item/components_l"},
		{"item/industrial_medicine_m", 4, "item/industrial_medicine_s"},
		{"item/wood_pile", 75, "item/wood_pile"},
	} {
		if got := SizedItemKey(c.key, c.qty); got != c.want {
			t.Errorf("SizedItemKey(%q, %d) = %q, want %q", c.key, c.qty, got, c.want)
		}
	}
	if ItemSprite("cloth carpet") != "item/carpet_cloth_rolled" || ItemSprite("granite chunk") != "item/stone_chunk" ||
		ItemSprite("marble blocks") != "item/stone_blocks" || ItemSprite("components") != "item/components_m" {
		t.Fatal("item keys")
	}
}

// TestPlantKeys: trees, crops and ground plants resolve through the key
// table; ground plants draw young, mature, harvestable or dormant.
func TestPlantKeys(t *testing.T) {
	t.Parallel()
	bush := game.PlantView{Species: "berry_bush", X: 1, Y: 2, Growth: 100}
	k, sc := PlantSprite(bush)
	if sc != 1 || k == "" || k != PlantKeys.Ground["berry_bush"].Mature[sprite.TileHash(1, 2)%GroundVariants] {
		t.Fatalf("mature berry bush = %q × %v", k, sc)
	}
	bush.Harvestable = true
	if k, _ = PlantSprite(bush); k != PlantKeys.Ground["berry_bush"].harvest[sprite.TileHash(1, 2)%GroundVariants] {
		t.Fatalf("ripe berry bush = %q", k)
	}
	bush.Wilted = true
	if k, _ = PlantSprite(bush); k != PlantKeys.Ground["berry_bush"].Dormant[sprite.TileHash(1, 2)%GroundVariants] {
		t.Fatalf("wilted berry bush = %q", k)
	}
	if k, sc = PlantSprite(game.PlantView{Species: "pine", Growth: 0}); k == "" || sc != 0.45 {
		t.Fatalf("young pine = %q × %v", k, sc)
	}
	if k, _ = PlantSprite(game.PlantView{Species: "jungle_tree"}); PlantKeys.Fallback[k] == nil {
		t.Fatalf("jungle tree %q has no oak fallback", k)
	}
	if k, _, ok := NewPlantKey(game.PlantView{Species: "healroot"}, 0); !ok || k != PlantKeys.Ground["healroot_wild"].Young[0] {
		t.Fatalf("wild healroot = %q %v", k, ok)
	}
	if _, _, ok := NewPlantKey(game.PlantView{Species: "unknown"}, 0); ok {
		t.Fatal("unknown species has a key")
	}
	if ClampStage(-1) != 0 || ClampStage(7) != 3 || ClampStage(2) != 2 {
		t.Fatal("ClampStage")
	}
	if c, s := PlantMarkColor(&game.PlantView{Species: "tree"}); s != 0.8 || c.G == 0 {
		t.Fatalf("tree mark %v %v", c, s)
	}
	if _, s := PlantMarkColor(&game.PlantView{Species: "dandelion"}); s != 0.4 {
		t.Fatalf("dandelion mark scale %v", s)
	}
}

func TestOnFloorKinds(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]bool{"sleeping spot": true, "carpet": true, "bridge": true, "power conduit": true, "hidden conduit": true, "bed": false, "wall": false} {
		if OnFloor(kind) != want {
			t.Errorf("OnFloor(%q) = %v", kind, !want)
		}
	}
}

// TestLabelsText: the room and fence hover lines, the interaction icons
// and how long a bubble lasts.
func TestLabelsText(t *testing.T) {
	t.Parallel()
	w := game.WorldView{TerrainW: 3, RoomGrid: []int{-1, 0, 0}, Rooms: []game.RoomView{{Role: "Bedroom", Label: "decent", Score: 12, Lit: true, Roofed: 1, Cleanliness: "dirty", FilthPenalty: 3}},
		Buildings: []game.BuildingView{{Kind: "fence", X: 1, Y: 0}, {Kind: "fence gate", X: 2, Y: 0}}}
	if got := RoomLabelAt(w, 0, 0); got != "Outdoors" {
		t.Fatalf("outdoors %q", got)
	}
	if got, want := RoomLabelAt(w, 1, 0), "Room: Bedroom, decent (12), dirty (-3), lit, roofed 100%"; got != want {
		t.Fatalf("room %q, want %q", got, want)
	}
	if RoomLabelAt(w, 5, 0) != "" || RoomLabelAt(game.WorldView{}, 0, 0) != "" {
		t.Fatal("off the map")
	}
	if FenceLabelAt(w, 1, 0) == "" || FenceLabelAt(w, 2, 0) == FenceLabelAt(w, 1, 0) || FenceLabelAt(w, 0, 0) != "" {
		t.Fatal("fence labels")
	}
	if InteractionIconKey("deep_talk") != "ui_icon/social_deep_talk" || InteractionIconKey("gossip") != "" {
		t.Fatal("interaction icons")
	}
	p := game.PawnView{Interaction: "insult", InteractTick: 1000}
	if !InteractionRecent(p, 1000+interactBubbleTicks) || InteractionRecent(p, 1001+interactBubbleTicks) || InteractionRecent(game.PawnView{}, 0) {
		t.Fatal("bubble lifetime")
	}
}

// TestCaveMouthFacing: stone under a natural roof opens toward the first
// open side; built roofs, rock and water are not open ground.
func TestCaveMouthFacing(t *testing.T) {
	t.Parallel()
	code := func(n string) uint8 {
		for i, m := range TerrainNames {
			if m == n {
				return uint8(i)
			}
		}
		t.Fatalf("no terrain %q", n)
		return 0
	}
	const mw = 3
	w := game.WorldView{TerrainW: mw, Terrain: make([]uint8, mw*3), RoofCells: make([]uint8, mw*3)}
	for i := range w.Terrain {
		w.Terrain[i] = code("rock")
	}
	w.Terrain[1*mw+1] = code("stone")
	w.RoofCells[1*mw+1] = uint8(overlay.RoofNatural)
	w.Terrain[2*mw+1] = code("soil")
	if got := CaveMouthFacing(w, 1, 1); got != "south" {
		t.Fatalf("mouth faces %q, want south", got)
	}
	w.Terrain[2*mw+1] = code("deep_water")
	if got := CaveMouthFacing(w, 1, 1); got != "" {
		t.Fatalf("water beside: %q", got)
	}
	if CaveMouthFacing(w, 0, 0) != "" {
		t.Fatal("rock is no mouth")
	}
}

// waveCItems are the DEF-2 weapon and armor item kinds (item.Kind names).
var waveCItems = []string{
	"mace", "gladius", "longsword", "short bow", "recurve bow", "greatbow", "pila",
	"revolver", "autopistol", "machine pistol", "assault rifle", "sniper rifle", "pump shotgun", "chain shotgun",
	"heavy smg", "lmg", "minigun", "frag grenades", "molotov cocktails", "EMP grenades",
	"incendiary launcher", "emp launcher",
	"mortar shells", "incendiary shells", "EMP shells", "smoke shells",
	"flak vest", "flak pants", "flak jacket", "plate armor", "marine armor", "marine helmet", "simple helmet", "advanced helmet",
}

// TestItemSpriteWaveCKinds (DEF-2): every new weapon and armor kind
// resolves to the item sprite whose metadata names it.
func TestItemSpriteWaveCKinds(t *testing.T) {
	t.Parallel()
	itemLine := regexp.MustCompile(`(?m)^item:\s*(.+)$`)
	for _, kind := range waveCItems {
		key := ItemSprite(kind)
		raw, err := os.ReadFile(filepath.Join("../../../../../../assets/metadata", key+".yaml"))
		if err != nil {
			t.Errorf("ItemSprite(%q) = %q: no metadata (%v)", kind, key, err)
			continue
		}
		m := itemLine.FindSubmatch(raw)
		if m == nil || strings.Trim(strings.TrimSpace(string(m[1])), `"`) != kind {
			t.Errorf("%s's metadata names %q, want %q", key, m, kind)
		}
	}
	if got := ItemSprite("EMP grenades"); got != "item/emp_grenades" {
		t.Errorf("EMP grenades -> %q", got)
	}
}

// TestItemSpriteWaveEKinds: the wave E item kinds resolve to the sprite
// whose metadata names them; a mechanoid wreck picks a corpse variant.
func TestItemSpriteWaveEKinds(t *testing.T) {
	t.Parallel()
	itemLine := regexp.MustCompile(`(?m)^item:\s*(.+)$`)
	for _, kind := range []string{"chemfuel", "wort", "fine meal", "lavish meal", "pemmican", "packaged survival meal", "kibble",
		"ash flake", "silverdust", "battle tonic", "reagent", "advanced components", "uranium", "plasteel", "reinforced barrel"} {
		key := ItemSprite(kind)
		raw, err := os.ReadFile(filepath.Join("../../../../../../assets/metadata", key+".yaml"))
		if err != nil {
			t.Errorf("ItemSprite(%q) = %q: no metadata (%v)", kind, key, err)
			continue
		}
		if m := itemLine.FindSubmatch(raw); m == nil || strings.Trim(strings.TrimSpace(string(m[1])), `"`) != kind {
			t.Errorf("%s's metadata names %q, want %q", key, m, kind)
		}
	}
	for _, sp := range []string{"harrow", "spindle", "bulwark", "quill"} {
		key := ItemSpriteSpecies(sp+" wreck", sp, 3, 4)
		if !strings.HasPrefix(key, "item/corpse_"+sp+"_v") {
			t.Errorf("%s wreck -> %q", sp, key)
		}
		if _, err := os.Stat(filepath.Join("../../../../../../assets/metadata", key+".yaml")); err != nil {
			t.Errorf("%s: %v", key, err)
		}
	}
}
