package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
)

// forbidden are RimWorld's creature names; prompts describe our creatures
// in our own words (AGENTS.md).
var forbidden = []string{"muffalo", "boomalope", "boomrat", "thrumbo", "alphabeaver", "megascarab", "megaspider", "spelopede", "warg", "megasloth", "rimworld"}

var groupNames = []string{"small_mammal", "large_mammal", "canid", "felid", "bovid", "bird_small", "bird_large", "reptile", "insect"}

// lengths are the per-event ranges in seconds (plan A3).
var lengths = map[string][2]float64{"call": {1.0, 2.5}, "attack": {0.5, 1.2}, "hurt": {0.4, 1.0}, "die": {1.0, 2.5}}

func TestPromptsCoverRoster(t *testing.T) {
	t.Parallel()
	p, err := loadPrompts("../../assets/source/audio/animal_prompts.json")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Sound{}
	for _, s := range p.Sounds {
		byID[s.ID] = s
	}
	want := map[string]bool{"animal.burst": true}
	var keys []string
	for _, sp := range animal.All() {
		keys = append(keys, sp.String())
	}
	// the art roster (animal_roster.py) must name the same species
	py, err := os.ReadFile("../../assets/source/blender/artlib/families/animal_roster.py")
	if err != nil {
		t.Fatal(err)
	}
	inPy := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s+Species\('(\w+)',`).FindAllStringSubmatch(string(py), -1) {
		inPy[m[1]] = true
	}
	if len(keys) != 61 || len(inPy) != 61 {
		t.Fatalf("roster has %d species in Go and %d in animal_roster.py, want 61", len(keys), len(inPy))
	}
	for _, k := range keys {
		if !inPy[k] {
			t.Errorf("%s is in the Go roster but not animal_roster.py", k)
		}
		for ev := range lengths {
			want["animal."+k+"."+ev] = true
		}
	}
	for _, g := range groupNames {
		for ev := range lengths {
			want["animal."+g+"."+ev] = true
		}
	}
	for id := range want {
		s, ok := byID[id]
		if !ok {
			t.Errorf("%s has no prompt", id)
			continue
		}
		if id == "animal.burst" {
			if s.Seconds != 1.5 {
				t.Errorf("burst is %v s, want 1.5", s.Seconds)
			}
			continue
		}
		r := lengths[eventOf(id)]
		if s.Seconds < r[0] || s.Seconds > r[1] {
			t.Errorf("%s is %v s, want %v-%v", id, s.Seconds, r[0], r[1])
		}
	}
	for _, sp := range animal.All() {
		if animal.RosterOf(sp).Archetype == animal.ArchBird && byID["animal."+sp.String()+".call"].Seconds > 1.5 {
			t.Errorf("bird %s call is over 1.5 s", sp)
		}
	}
	for _, s := range p.Sounds {
		if !want[s.ID] {
			t.Errorf("%s is not a roster, group or burst sound", s.ID)
		}
		low := strings.ToLower(s.Prompt)
		for _, bad := range forbidden {
			if strings.Contains(low, bad) {
				t.Errorf("%s uses %q", s.ID, bad)
			}
		}
		if len(strings.Fields(s.Prompt)) < 3 {
			t.Errorf("%s prompt is too vague: %q", s.ID, s.Prompt)
		}
	}
	if len(p.Sounds) != 61*4+9*4+1 {
		t.Errorf("%d prompts, want %d", len(p.Sounds), 61*4+9*4+1)
	}
}
