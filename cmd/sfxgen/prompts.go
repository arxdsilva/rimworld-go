package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path"
	"regexp"
	"strings"
)

// Sound is one prompt row.
type Sound struct {
	ID      string  `json:"id"`
	Seconds float64 `json:"seconds"`
	Prompt  string  `json:"prompt"`
}

// Prompts is animal_prompts.json.
type Prompts struct {
	Version  int    `json:"version"`
	Note     string `json:"note"`
	Defaults struct {
		PromptInfluence float64 `json:"prompt_influence"`
		Suffix          string  `json:"suffix"`
	} `json:"defaults"`
	Sounds []Sound `json:"sounds"`
}

// eventSpec is how an event is mixed: loudness target (LUFS), manifest
// volume, and whether it gets a pitched B variant.
type eventSpec struct {
	lufs, volume float64
	variant      bool
}

var events = map[string]eventSpec{
	"call":   {-20, 0.7, true},
	"attack": {-18, 0.85, true},
	"hurt":   {-18, 0.85, true},
	"die":    {-19, 1.0, false},
	"burst":  {-16, 1.0, false},
}

var idPattern = regexp.MustCompile(`^animal\.[a-z0-9_]+(\.[a-z]+)?$`)

// loadPrompts reads and validates the prompt list; unknown fields and
// versions are refused so a schema change cannot be spent on silently.
func loadPrompts(file string) (Prompts, error) {
	var p Prompts
	raw, err := os.ReadFile(file)
	if err != nil {
		return p, err
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return p, fmt.Errorf("%s: %w", file, err)
	}
	if p.Version != 1 {
		return p, fmt.Errorf("%s: unknown schema version %d (want 1)", file, p.Version)
	}
	if p.Defaults.PromptInfluence < 0 || p.Defaults.PromptInfluence > 1 {
		return p, fmt.Errorf("%s: prompt_influence %v outside [0, 1]", file, p.Defaults.PromptInfluence)
	}
	seen := map[string]bool{}
	for _, s := range p.Sounds {
		switch {
		case !idPattern.MatchString(s.ID):
			return p, fmt.Errorf("%s: bad id %q", file, s.ID)
		case seen[s.ID]:
			return p, fmt.Errorf("%s: %s listed twice", file, s.ID)
		case strings.TrimSpace(s.Prompt) == "":
			return p, fmt.Errorf("%s: %s has no prompt", file, s.ID)
		case s.Seconds < 0.5 || s.Seconds > 30:
			return p, fmt.Errorf("%s: %s seconds %v outside the API's [0.5, 30]", file, s.ID, s.Seconds)
		}
		if _, ok := events[eventOf(s.ID)]; !ok {
			return p, fmt.Errorf("%s: %s has an unknown event %q", file, s.ID, eventOf(s.ID))
		}
		seen[s.ID] = true
	}
	return p, nil
}

// text is what the API receives for s.
func (p Prompts) text(s Sound) string { return s.Prompt + p.Defaults.Suffix }

// eventOf is the last part of the id: animal.cow.call -> call, animal.burst -> burst.
func eventOf(id string) string {
	i := strings.LastIndexByte(id, '.')
	return id[i+1:]
}

// stemOf is the file name of an id: animal.timber_wolf.call -> timber_wolf_call.
func stemOf(id string) string {
	return strings.ReplaceAll(strings.TrimPrefix(id, "animal."), ".", "_")
}

// matchIDs reports whether id matches any comma-separated id or glob.
func matchIDs(list, id string) bool {
	for _, pat := range strings.Split(list, ",") {
		if pat = strings.TrimSpace(pat); pat == "" {
			continue
		}
		if ok, _ := path.Match(pat, id); ok || pat == id {
			return true
		}
	}
	return false
}

// selectSounds filters by -only (all when empty); a pattern matching
// nothing is an error, since a typo must not look like an empty run.
func selectSounds(all []Sound, only string) ([]Sound, error) {
	if strings.TrimSpace(only) == "" {
		return all, nil
	}
	var out []Sound
	for _, s := range all {
		if matchIDs(only, s.ID) {
			out = append(out, s)
		}
	}
	for _, pat := range strings.Split(only, ",") {
		pat = strings.TrimSpace(pat)
		hit := false
		for _, s := range all {
			if matchIDs(pat, s.ID) {
				hit = true
				break
			}
		}
		if pat != "" && !hit {
			return nil, fmt.Errorf("-only %q matches no sound in the prompt list", pat)
		}
	}
	return out, nil
}

// estimate is the credits seconds of sound cost at rate credits a second.
func estimate(seconds, rate float64) int {
	return int(math.Ceil(seconds*rate - 1e-9))
}

// variantRate is the pitch factor of an id's B variant: +4% or -4% by hash.
func variantRate(id string) float64 {
	h := fnv.New32a()
	h.Write([]byte(id))
	if h.Sum32()&1 == 0 {
		return 1.04
	}
	return 0.96
}

// serialOf is a stable Ogg stream serial for id and variant (oggenc
// picks a random one otherwise, which would make the output vary).
func serialOf(id, variant string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(id + "/" + variant))
	return h.Sum32() & 0x7fffffff
}
