package render

import (
	"image"
	"image/color"
	"math"
	"testing"

	"rimworld-go/internal/domain/world/biome"
	"rimworld-go/internal/domain/world/geology"
	"rimworld-go/internal/domain/world/mapgen"
	"rimworld-go/internal/domain/world/mapgen/pipeline"
	"rimworld-go/internal/domain/world/plant"
	"rimworld-go/internal/domain/world/roof"
	"rimworld-go/internal/domain/world/terrain"
)

func TestPalettesCoverEverything(t *testing.T) {
	magenta := color.RGBA{255, 0, 255, 255}
	seen := map[color.RGBA]string{}
	for i, n := range terrain.Names() {
		c := TerrainColor(terrain.Type(i))
		if c == magenta {
			t.Errorf("terrain %s has no colour", n)
		}
		if prev, dup := seen[c]; dup {
			t.Errorf("terrain %s and %s share a colour", n, prev)
		}
		seen[c] = n
	}
	if TerrainColor(terrain.Type(200)) != magenta {
		t.Error("unknown terrain not magenta")
	}
	for _, s := range geology.Stones() {
		if c := StoneColor(s); c == magenta || c == StoneColor(geology.None) {
			t.Errorf("stone %v has no colour", s)
		}
	}
	if StoneColor(99) != magenta {
		t.Error("unknown stone")
	}
	for _, o := range append(geology.Ores(), geology.Machinery) {
		if OreColor(o) == magenta {
			t.Errorf("ore %v", o)
		}
	}
	if OreColor(geology.NoOre) != magenta {
		t.Error("no-ore colour")
	}
	if PlantColor(plant.Tree) == PlantColor(plant.Shrub) || PlantColor(plant.Shrub) == PlantColor(plant.Moss) {
		t.Error("plant groups must differ")
	}
}

func TestRampsSurviveBadValues(t *testing.T) {
	inf, nan := float32(math.Inf(1)), float32(math.NaN())
	for _, d := range []float32{inf, nan, mapgen.Far, -1} {
		if DistRamp(d) != Background {
			t.Errorf("DistRamp(%v) should be background", d)
		}
	}
	if DistRamp(0) == Background || DistRamp(10) == DistRamp(50) {
		t.Error("near distances must differ")
	}
	for _, v := range []float32{-1, 0, 0.2, 0.5, 0.9, 1, 2, nan, inf} {
		Grey(v)
		WetRamp(v)
		Heat(v)
	}
	if Grey(nan) != Grey(0) || Grey(2) != Grey(1) {
		t.Error("clamp")
	}
	if RegionColor(-1) == RegionColor(0) || RegionColor(1) == RegionColor(2) {
		t.Error("region colours")
	}
}

func TestDigitsAndUpscale(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 60, 20))
	DrawNumber(img, 1, 1, 1234567890, 1) // clipped at the right edge, must not panic
	DrawNumber(img, 0, 0, 7, 0)
	white := 0
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i] == 255 {
			white++
		}
	}
	if white == 0 {
		t.Fatal("nothing drawn")
	}
	if TextWidth(0, 1) != 0 || TextWidth(3, 2) != (3*6-1)*2 {
		t.Fatal("TextWidth")
	}
	u := Upscale(img, 3)
	if u.Bounds().Dx() != 180 || u.Bounds().Dy() != 60 {
		t.Fatal("upscale size")
	}
	if Upscale(img, 1) != img {
		t.Fatal("scale 1 should be identity")
	}
	src := image.NewRGBA(image.Rect(0, 0, 2, 1))
	src.SetRGBA(1, 0, color.RGBA{9, 8, 7, 255})
	if got := Upscale(src, 2).RGBAAt(3, 1); got != (color.RGBA{9, 8, 7, 255}) {
		t.Fatalf("pixel %v", got)
	}
}

func TestSheetSize(t *testing.T) {
	if w, h := SheetSize(0, 10, 10); w != 0 || h != 0 {
		t.Fatal("empty")
	}
	if w, h := SheetSize(2, 10, 10); w != 20+SheetGap || h != 10 {
		t.Fatalf("%d %d", w, h)
	}
	if w, h := SheetSize(5, 10, 10); w != 40+3*SheetGap || h != 20+SheetGap {
		t.Fatalf("%d %d", w, h)
	}
	tile := image.NewRGBA(image.Rect(0, 0, 10, 10))
	s := Sheet([]*image.RGBA{tile, tile, tile}, []uint64{1, 2}, 10, 10, 1)
	if s.Bounds().Dx() != 30+2*SheetGap {
		t.Fatal("sheet width")
	}
}

func TestFieldsFromFullMap(t *testing.T) {
	cfg := mapgen.Config{Seed: 3, W: 40, H: 40, Biome: biome.DefOf(biome.TemperateForest), Debug: true}
	res, err := pipeline.Generate(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Hand-fill features so every painter has something to draw whatever
	// the steps currently do.
	res.Plants = append(res.Plants, mapgen.PlantSpawn{X: 1, Y: 1, Species: plant.Tree}, mapgen.PlantSpawn{X: -5, Y: 1})
	res.Chunks = append(res.Chunks, mapgen.ChunkSpawn{X: 2, Y: 2})
	res.Vents = append(res.Vents, mapgen.Point{X: 39, Y: 39})
	res.Ruins = append(res.Ruins, mapgen.Ruin{Walls: []mapgen.RuinWall{{X: 3, Y: 3}}, Floors: []mapgen.RuinFloor{{X: 4, Y: 3}}})
	res.Fields.Road[5] = true
	res.Fields.RiverLine = []mapgen.PointF{{X: 10.5, Y: 10.5}, {X: -3, Y: 100}}
	res.Fields.RiverDist[0], res.Fields.CoastDist[1] = float32(math.Inf(1)), 3
	res.Roof[0], res.Roof[1] = roof.Natural, roof.Natural|roof.NaturalThick
	res.Terrain.Cells[2] = terrain.Rock
	res.Fields.Cave[3], res.Fields.Rock[4] = true, true
	res.Geology.Ore[6] = geology.Gold
	res.Terrain.Cells[6] = terrain.Rock
	res.Start = mapgen.Point{X: 0, Y: 0}
	imgs, err := Fields(res)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != len(Names) {
		t.Fatalf("%d images, want %d", len(imgs), len(Names))
	}
	for i, n := range imgs {
		if n.Name != Names[i] || n.Img.Bounds().Dx() != 40 || n.Img.Bounds().Dy() != 40 {
			t.Errorf("image %d: %s %v", i, n.Name, n.Img.Bounds())
		}
	}
	if c, err := Composite(res); err != nil || c.Bounds().Dx() != 40 {
		t.Fatal("composite", err)
	}
	// Start off the map must not panic.
	res.Start = mapgen.Point{X: -1, Y: 99}
	if _, err := Fields(res); err != nil {
		t.Fatal(err)
	}
}

func TestFieldsNeedDebug(t *testing.T) {
	res, err := pipeline.Generate(mapgen.Config{Seed: 1, W: 24, H: 24, Biome: biome.DefOf(biome.TemperateForest)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Fields(res); err != ErrNoFields {
		t.Fatalf("got %v", err)
	}
	if _, err := Composite(res); err != ErrNoFields {
		t.Fatalf("got %v", err)
	}
}
