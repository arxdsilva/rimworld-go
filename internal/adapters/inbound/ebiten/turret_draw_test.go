package ebiten

import (
	"fmt"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/sim/entity"
)

// PLAY-024 renderer: turret gun frames, glows, muzzle flash, beacon
// radius, inspector line, trade alerts.

// turretArt adds the turret, console and beacon sprites to pdArt.
func turretArt() *artCatalog {
	c := pdArt()
	c.Sprites[turretBaseKey] = fakeSprite([2]int{160, 200}, pt{16, 80}, map[string]pt{"mount": {80, 100}})
	c.Sprites[turretBaseKey+"_glow"] = fakeSprite([2]int{160, 200}, pt{16, 80}, nil)
	for i := 0; i < turretAimFrames; i++ {
		c.Sprites[turretGunKey(i)] = fakeSprite([2]int{120, 120}, pt{20, 60}, map[string]pt{"mount": {60, 60}, "muzzle": {60 + float64(i), 110}})
	}
	c.Sprites["power/trade_beacon"] = fakeSprite([2]int{160, 200}, pt{16, 80}, nil)
	c.Sprites["power/trade_beacon_glow"] = fakeSprite([2]int{160, 200}, pt{16, 80}, nil)
	return c
}

func turretRenderer(t *testing.T, b game.BuildingView) (*Renderer, *[]string) {
	t.Helper()
	w := game.WorldView{Buildings: []game.BuildingView{b}}
	r, got := pdRenderer(t, w)
	r.art = turretArt()
	return r, got
}

func drawOne(r *Renderer) {
	r.drawBuildingObj(ebiten.NewImage(640, 400), &r.view.W.Buildings[0], r.tilePx(), 640, 400)
}

func TestTurretDrawAimFrame(t *testing.T) {
	t.Parallel()
	for _, dir := range []int8{0, 5, 15} {
		b := game.BuildingView{ID: 1, Kind: sentryTurretKind, X: 3, Y: 3, AimDir: dir, ShotAge: -1, PowerRole: "consumer", Powered: true}
		r, got := turretRenderer(t, b)
		drawOne(r)
		if want := "gun " + fmt.Sprintf("defense/sentry_turret_gun_%02d", dir); !pdHas(*got, want) {
			t.Errorf("aim %d drew %v, want %q", dir, *got, want)
		}
	}
	if recoilPush(-1) != 0 || recoilPush(0) <= recoilPush(3) || recoilPush(turretRecoilTicks) != 0 {
		t.Error("recoil should push at the shot, fade and end")
	}
	if v := turretAimVec(0); v.x != 0 || v.y != 1 {
		t.Errorf("frame 0 should point south, got %v", v)
	}
	if v := turretAimVec(4); v.x > -0.99 {
		t.Errorf("frame 4 should point west (clockwise on screen), got %v", v)
	}
}

func TestTurretGlowOnlyWhenPowered(t *testing.T) {
	t.Parallel()
	for _, powered := range []bool{true, false} {
		b := game.BuildingView{ID: 1, Kind: sentryTurretKind, X: 3, Y: 3, ShotAge: -1, PowerRole: "consumer", Powered: powered}
		r, got := turretRenderer(t, b)
		drawOne(r)
		if has := pdHas(*got, "glow "+turretBaseKey+"_glow"); has != powered {
			t.Errorf("powered %v: glow drawn %v (%v)", powered, has, *got)
		}
	}
}

func TestTurretMuzzleFlashFromBuildingTrace(t *testing.T) {
	t.Parallel()
	b := game.BuildingView{ID: 7, Kind: sentryTurretKind, X: 3, Y: 3, AimDir: 2, ShotAge: 0}
	r, _ := turretRenderer(t, b)
	tr := game.TraceView{FromX: 3, FromY: 3, ToX: 8, ToY: 3, FromBuilding: true, Attacker: entity.EntityID(7)}
	mz, ok := r.traceMuzzle(tr)
	if !ok {
		t.Fatal("no muzzle for a turret's trace")
	}
	// mount (80,100) on origin (16,80), gun mount (60,60) origin (20,60), muzzle (62,110)
	px := float64(artTilePx)
	wantX := 3 + (80-16)/px - (60-20)/px + (62-20)/px
	wantY := 3 + (100-80)/px - (60-60)/px + (110-60)/px
	if d := mz.x - wantX; d > 1e-9 || d < -1e-9 {
		t.Errorf("muzzle x %v, want %v", mz.x, wantX)
	}
	if d := mz.y - wantY; d > 1e-9 || d < -1e-9 {
		t.Errorf("muzzle y %v, want %v", mz.y, wantY)
	}
	if _, ok := r.traceMuzzle(game.TraceView{FromBuilding: true, Attacker: 99}); ok {
		t.Error("an unknown turret has no muzzle")
	}
}

func TestBeaconGhostShowsRadius(t *testing.T) {
	t.Parallel()
	if rad, ok := placementRing(tradeBeaconKind); !ok || rad != 6 {
		t.Fatalf("beacon ring = %v %v, want 6", rad, ok)
	}
	if _, ok := placementRing("wall"); ok {
		t.Error("only the beacon shows a ghost ring")
	}
	r, got := turretRenderer(t, game.BuildingView{ID: 1, Kind: tradeBeaconKind, X: 2, Y: 2})
	r.drawRangeRing(ebiten.NewImage(640, 400), 4, 5, beaconRadiusTiles, tokenNRGBA(uiAccent))
	if !pdHas(*got, "ring 4,5,6") {
		t.Errorf("ring not drawn: %v", *got)
	}
	// the beacon light blinks (and stays lit under reduced motion)
	on, off := 0, 0
	for f := uint64(0); f < beaconBlinkFrames; f++ {
		if beaconLit(f, 1, false) {
			on++
		} else {
			off++
		}
	}
	if on == 0 || off == 0 || !beaconLit(1000, 1, true) {
		t.Errorf("beacon blink on %d off %d", on, off)
	}
}

func TestTurretInspectorStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		b    game.BuildingView
		want string
	}{
		{game.BuildingView{Kind: sentryTurretKind, Powered: false, ShotAge: 10}, "No power"},
		{game.BuildingView{Kind: sentryTurretKind, Powered: true, ShotAge: -1}, "Idle"},
		{game.BuildingView{Kind: sentryTurretKind, Powered: true, ShotAge: 30}, "Target locked"},
		{game.BuildingView{Kind: sentryTurretKind, Powered: true, ShotAge: 500}, "Idle"},
		{game.BuildingView{Kind: "wall"}, ""},
	}
	for _, c := range cases {
		if got := turretLine(c.b); got != c.want {
			t.Errorf("%+v: %q, want %q", c.b, got, c.want)
		}
	}
}

type fakeTradeStatus struct {
	o game.OrbitalView
	c bool
}

func (f fakeTradeStatus) TradeStatus() (game.OrbitalView, bool) { return f.o, f.c }

func TestTradeAlerts(t *testing.T) {
	t.Parallel()
	got := tradeAlertsFrom(fakeTradeStatus{game.OrbitalView{Present: true, HoursLeft: 18, CaravanVisiting: true}, true})
	if len(got) != 2 || got[0].label != "Trade ship in orbit (18h left)" || got[1].label != "Trader caravan visiting" {
		t.Fatalf("alerts = %+v", got)
	}
	for _, a := range got {
		if a.danger {
			t.Errorf("%s should not be a danger", a.key)
		}
		if e := alertExplain(a.key); e == "Select this alert to look at the cause." {
			t.Errorf("%s has no explanation", a.key)
		}
	}
	if n := len(tradeAlertsFrom(fakeTradeStatus{})); n != 0 {
		t.Errorf("nothing visiting raised %d alerts", n)
	}
	if n := len(tradeAlertsFrom(struct{}{})); n != 0 {
		t.Errorf("a source with no status raised %d alerts", n)
	}
}

// TestTurretArtCoversRenderer: every sprite the PLAY-024 renderer asks
// for is generated and the glow/attachment names match. Skips until the
// art lands.
func TestTurretArtCoversRenderer(t *testing.T) {
	t.Parallel()
	root, err := LocateArt()
	if err != nil {
		t.Skip(err)
	}
	if _, ok := pdMeta(t, root, turretBaseKey); !ok {
		t.Skip("PLAY-024 art not generated yet")
	}
	keys := []string{turretBaseKey, glowKey(turretBaseKey), "power/trade_beacon", glowKey("power/trade_beacon")}
	for i := 0; i < turretAimFrames; i++ {
		keys = append(keys, turretGunKey(i))
	}
	for _, f := range facingNames {
		keys = append(keys, buildSprite(radioConsoleKind, f, 0, false, false), glowKey(buildSprite(radioConsoleKind, f, 0, false, false)))
	}
	for _, tool := range archTools {
		if tool.build == sentryTurretKind || tool.build == tradeBeaconKind || tool.build == radioConsoleKind {
			keys = append(keys, "ui_icon/"+tool.icon)
		}
	}
	for _, k := range keys {
		if _, ok := pdMeta(t, root, k); !ok {
			t.Errorf("%s: no generated sprite", k)
		}
	}
}
