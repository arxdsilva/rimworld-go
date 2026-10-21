package audiomanifest

import (
	"bytes"
	"testing"
)

func TestRoundTripSortedAndValid(t *testing.T) {
	t.Parallel()
	m := Manifest{Sounds: []Entry{
		{ID: "ui.click", Bus: BusUI, Volume: 0.6, Files: []string{"ui/click_0.wav"}, Source: SourceProcedural, License: "CC0"},
		{ID: "amb.day", Bus: BusAmbience, Loop: true, Volume: 0.8, Files: []string{"ambience/day.ogg"}, Source: SourceElevenLabs, License: "x"},
	}}
	var buf bytes.Buffer
	if err := Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	got, err := Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || len(got.Sounds) != 2 || got.Sounds[0].ID != "amb.day" {
		t.Fatalf("got %+v", got)
	}
	if e, ok := got.Lookup("ui.click"); !ok || e.Files[0] != "ui/click_0.wav" {
		t.Fatal("lookup")
	}
}

func TestValidateRejects(t *testing.T) {
	t.Parallel()
	good := Entry{ID: "sfx.chop", Bus: BusEffects, Volume: 1, Files: []string{"sfx/chop_0.wav"}, Source: SourceProcedural, License: "CC0"}
	bad := []func(e *Entry){
		func(e *Entry) { e.Bus = "loud" },
		func(e *Entry) { e.Files = nil },
		func(e *Entry) { e.License = "" },
		func(e *Entry) { e.Volume = 2 },
		func(e *Entry) { e.Files = []string{"../../etc/passwd"} },
	}
	for i, f := range bad {
		e := good
		f(&e)
		if err := (Manifest{Sounds: []Entry{e}}).Validate(); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
	if err := (Manifest{Sounds: []Entry{good, good}}).Validate(); err == nil {
		t.Fatal("duplicate id accepted")
	}
}

func TestBusOf(t *testing.T) {
	t.Parallel()
	for id, want := range map[string]string{"ui.click": BusUI, "sfx.chop": BusEffects, "amb.rain": BusAmbience, "music.calm_1": BusMusic} {
		if got := BusOf(id); got != want {
			t.Fatalf("BusOf(%s) = %s, want %s", id, got, want)
		}
	}
}
