package config

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestSettingsRoundTrip(t *testing.T) {
	t.Parallel()
	s := Defaults()
	s.Audio.Music = 30
	s.Graphics.Effects = EffectsMinimal
	s.Gameplay.AutosaveDays = 7
	s.Accessibility.ReducedMotion = true
	var buf bytes.Buffer
	if err := Encode(&buf, s); err != nil {
		t.Fatal(err)
	}
	got, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, s) {
		t.Fatalf("round trip\n got %+v\nwant %+v", got, s)
	}
}

func TestSettingsPartialKeepsDefaults(t *testing.T) {
	t.Parallel()
	got, err := Decode(strings.NewReader(`{"version": 1, "audio": {"music": 20}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := Defaults()
	want.Audio.Music = 20
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestSettingsNormalize(t *testing.T) {
	t.Parallel()
	s := Defaults()
	s.Audio.Master = 147
	s.Audio.UI = 34
	s.Graphics.Effects = "ultra"
	s.Gameplay.AutosaveDays = 5
	s.Gameplay.PanSpeed = 10
	s.Version = 99
	n := s.Normalize()
	if n.Audio.Master != 100 || n.Audio.UI != 30 || n.Graphics.Effects != EffectsFull ||
		n.Gameplay.AutosaveDays != 1 || n.Gameplay.PanSpeed != PanSpeedMin || n.Version != SettingsVersion {
		t.Fatalf("normalize = %+v", n)
	}
	if !reflect.DeepEqual(Defaults().Normalize(), Defaults()) {
		t.Fatal("defaults are not normal")
	}
}

func TestSettingsDecodeErrorGivesDefaults(t *testing.T) {
	t.Parallel()
	got, err := Decode(strings.NewReader(`{not json`))
	if err == nil || !reflect.DeepEqual(got, Defaults()) {
		t.Fatalf("got %+v, %v; want defaults and an error", got, err)
	}
}

func TestGainAndFX(t *testing.T) {
	t.Parallel()
	if Gain(0) != 0 || Gain(100) != 1 || Gain(50) != 0.25 || Gain(140) != 1 {
		t.Fatal("gain curve")
	}
	if (Graphics{Effects: EffectsFull}).FX() != 1 || (Graphics{Effects: EffectsMinimal}).FX() != 0.2 {
		t.Fatal("fx levels")
	}
}

func TestControlsAndDevModeDecodeOverDefaults(t *testing.T) {
	t.Parallel()
	got, err := Decode(strings.NewReader(`{"version": 1, "gameplay": {"dev_mode": true}, "controls": {"bindings": {"thing.draft": [" Y ", "Y", ""]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Gameplay.DevMode || got.Gameplay.PanSpeed != 100 {
		t.Fatalf("gameplay = %+v", got.Gameplay)
	}
	if b := got.Controls.Bindings["thing.draft"]; len(b) != 1 || b[0] != "Y" {
		t.Fatalf("bindings = %v", got.Controls.Bindings)
	}
	old, err := Decode(strings.NewReader(`{"version": 1, "audio": {"music": 20}}`))
	if err != nil || old.Controls.Bindings != nil || old.Gameplay.DevMode {
		t.Fatalf("an old file gets default controls: %+v %v", old.Controls, err)
	}
}

func TestUIScalePresetsSnap(t *testing.T) {
	t.Parallel()
	for in, want := range map[int]int{0: 100, 50: 75, 75: 75, 80: 75, 85: 90, 96: 100, 100: 100, 107: 100, 110: 115, 120: 115, 125: 130, 140: 130, 141: 150, 160: 150, 165: 175, 190: 200, 250: 200} {
		s := Defaults()
		s.Accessibility.UIScale = in
		if got := s.Normalize().Accessibility.UIScale; got != want {
			t.Errorf("UIScale %d -> %d, want %d", in, got, want)
		}
	}
	if UIScalePresets[0] != UIScaleMin || UIScalePresets[len(UIScalePresets)-1] != UIScaleMax {
		t.Fatalf("presets %v do not span %d..%d", UIScalePresets, UIScaleMin, UIScaleMax)
	}
}

func TestUIScaleAutoAndHiDPIDefaultOn(t *testing.T) {
	t.Parallel()
	d := Defaults()
	if !d.Accessibility.UIScaleAuto || !d.Graphics.HiDPI {
		t.Fatalf("defaults: auto %v hidpi %v, want both on", d.Accessibility.UIScaleAuto, d.Graphics.HiDPI)
	}
	// An old (version 1) file has neither field and an explicit 100 %:
	// it loads with Auto and HiDPI on.
	old, err := Decode(strings.NewReader(`{"version": 1, "accessibility": {"ui_scale": 100}, "graphics": {"vsync": true}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !old.Accessibility.UIScaleAuto || !old.Graphics.HiDPI {
		t.Fatalf("old file: auto %v hidpi %v, want both on", old.Accessibility.UIScaleAuto, old.Graphics.HiDPI)
	}
	// The zero value (a hand-built Settings) migrates the same way.
	if z := (Settings{}).Normalize(); !z.Accessibility.UIScaleAuto || !z.Graphics.HiDPI || z.Accessibility.UIScale != 100 {
		t.Fatalf("zero settings normalize to %+v", z.Accessibility)
	}
	// A current file keeps an explicit off.
	cur, err := Decode(strings.NewReader(`{"version": 2, "accessibility": {"ui_scale": 150, "ui_scale_auto": false}, "graphics": {"hidpi": false}}`))
	if err != nil {
		t.Fatal(err)
	}
	if cur.Accessibility.UIScaleAuto || cur.Graphics.HiDPI || cur.Accessibility.UIScale != 150 {
		t.Fatalf("current file: %+v %+v", cur.Accessibility, cur.Graphics)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, cur); err != nil {
		t.Fatal(err)
	}
	if back, err := Decode(&buf); err != nil || !reflect.DeepEqual(back, cur) {
		t.Fatalf("round trip %+v, %v", back, err)
	}
}

// PLAY-017: Gameplay.RunInBackground defaults off, a version 2 file loads
// with it off (and its other choices kept), and an explicit on survives a
// round trip.
func TestRunInBackgroundDefaultsOffAndMigrates(t *testing.T) {
	t.Parallel()
	if Defaults().Gameplay.RunInBackground {
		t.Fatal("run in background must default off")
	}
	old, err := Decode(strings.NewReader(`{"version": 2, "gameplay": {"autosave_days": 7, "pause_on_threat": false}}`))
	if err != nil {
		t.Fatal(err)
	}
	if old.Gameplay.RunInBackground || old.Gameplay.AutosaveDays != 7 || old.Gameplay.PauseOnThreat {
		t.Fatalf("version 2 file: %+v, want RunInBackground off and the rest kept", old.Gameplay)
	}
	if old.Version != SettingsVersion || SettingsVersion < 3 {
		t.Fatalf("migrated version %d, current %d, want the PLAY-017 bump (>= 3)", old.Version, SettingsVersion)
	}
	cur, err := Decode(strings.NewReader(`{"version": 3, "gameplay": {"run_in_background": true}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !cur.Gameplay.RunInBackground {
		t.Fatalf("current file lost run_in_background: %+v", cur.Gameplay)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, cur); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"run_in_background": true`) {
		t.Fatalf("encoded settings do not persist the option:\n%s", buf.String())
	}
	if back, err := Decode(&buf); err != nil || !back.Gameplay.RunInBackground {
		t.Fatalf("round trip %+v, %v", back.Gameplay, err)
	}
}

// PLAY-019: the low-RAM notice remembers the RAM amount it was dismissed
// for (0 = never), and the value survives a round trip.
func TestRAMNoticeSeenRoundTrip(t *testing.T) {
	t.Parallel()
	if Defaults().Gameplay.RAMNoticeSeen != 0 {
		t.Fatal("the low-RAM notice must start unseen")
	}
	s := Defaults()
	s.Gameplay.RAMNoticeSeen = 4 << 30
	var buf strings.Builder
	if err := Encode(&buf, s); err != nil {
		t.Fatal(err)
	}
	got, err := Decode(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	if got.Gameplay.RAMNoticeSeen != 4<<30 {
		t.Fatalf("RAMNoticeSeen %d after a round trip, want %d", got.Gameplay.RAMNoticeSeen, uint64(4<<30))
	}
}
