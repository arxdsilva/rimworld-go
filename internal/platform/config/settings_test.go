package config

import (
	"bytes"
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
	if got != s {
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
	if got != want {
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
	if Defaults().Normalize() != Defaults() {
		t.Fatal("defaults are not normal")
	}
}

func TestSettingsDecodeErrorGivesDefaults(t *testing.T) {
	t.Parallel()
	got, err := Decode(strings.NewReader(`{not json`))
	if err == nil || got != Defaults() {
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
