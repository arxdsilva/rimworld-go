package config

import (
	"reflect"
	"strings"
	"testing"
)

// A version 9 file keeps what PauseOnThreat and ReducedMotion decided.
func TestPrefsMigrateFromVersion9(t *testing.T) {
	for _, c := range []struct {
		json        string
		pause       string
		smoothJumps bool
	}{
		{`{"version":9,"gameplay":{"pause_on_threat":true}}`, AutoPauseMajor, true},
		{`{"version":9,"gameplay":{"pause_on_threat":false},"accessibility":{"reduced_motion":true}}`, AutoPauseNever, false},
	} {
		s, err := Decode(strings.NewReader(c.json))
		if err != nil {
			t.Fatal(err)
		}
		if s.General.AutoPause != c.pause || s.Camera.SmoothJumps != c.smoothJumps || s.Version != SettingsVersion {
			t.Errorf("%s: auto pause %q smooth %v version %d", c.json, s.General.AutoPause, s.Camera.SmoothJumps, s.Version)
		}
	}
}

// Out-of-range and unknown values fall back to their defaults or clamp.
func TestPrefsNormalize(t *testing.T) {
	s := Defaults()
	s.General.AutoPause, s.General.AutosavesKept, s.General.Language = "sometimes", 99, ""
	s.Interface.FarStyle, s.Interface.TooltipDelayMS = "neon", 5000
	s.Camera.DragPct, s.Camera.ShakePct, s.Camera.Window = 10, -5, "7x7"
	s.Gameplay.TempUnit = "R"
	n := s.Normalize()
	d := Defaults()
	if n.General.AutoPause != d.General.AutoPause || n.General.AutosavesKept != AutosavesKeptMax || n.General.Language != "en" {
		t.Errorf("general %+v", n.General)
	}
	if n.Interface.FarStyle != FarDots || n.Interface.TooltipDelayMS != TooltipDelayMax {
		t.Errorf("interface %+v", n.Interface)
	}
	if n.Camera.DragPct != DragMin || n.Camera.ShakePct != 0 || n.Camera.Window != "" || n.Gameplay.TempUnit != "" {
		t.Errorf("camera %+v unit %q", n.Camera, n.Gameplay.TempUnit)
	}
	if k := Defaults(); !reflect.DeepEqual(k.Normalize().General, k.General) || k.Normalize().Interface != k.Interface || k.Normalize().Camera != k.Camera {
		t.Error("defaults change under Normalize")
	}
}
