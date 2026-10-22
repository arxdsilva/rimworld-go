package scenario

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/pawngen"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/research"
)

// FileVersion is the scenario file format version.
const FileVersion = 1

// The file is {"version":1,"def":{...}}. Items, traits, species,
// buildings, research projects, incidents, conditions and enums are all
// stored by stable string keys, never numbers, so reordering the Go
// enums can never change what a saved scenario means.

type fileEnvelope struct {
	Version int             `json:"version"`
	Def     json.RawMessage `json:"def"`
}

type fileDef struct {
	Key      string            `json:"key,omitempty"`
	Name     string            `json:"name"`
	Summary  string            `json:"summary,omitempty"`
	Desc     string            `json:"desc,omitempty"`
	Seed     uint64            `json:"seed,omitempty"`
	Faction  string            `json:"faction"`
	People   int               `json:"people"`
	Choose   int               `json:"choose_from"`
	Arrival  string            `json:"arrival"`
	Tech     string            `json:"tech"`
	Hardness uint8             `json:"hardness,omitempty"`
	Parts    []json.RawMessage `json:"parts"`
}

type filePart struct {
	Kind      string `json:"kind"`
	Trait     string `json:"trait,omitempty"`
	Chance    uint8  `json:"chance,omitempty"`
	Who       string `json:"who,omitempty"`
	N         uint8  `json:"n,omitempty"`
	MinAge    uint8  `json:"min_age,omitempty"`
	MaxAge    uint8  `json:"max_age,omitempty"`
	PawnCond  string `json:"pawn_condition,omitempty"`
	Item      string `json:"item,omitempty"`
	Qty       int32  `json:"qty,omitempty"`
	Near      bool   `json:"near,omitempty"`
	Species   string `json:"species,omitempty"`
	Random    bool   `json:"random,omitempty"`
	Cond      string `json:"condition,omitempty"`
	Incident  string `json:"incident,omitempty"`
	FirstDay  uint16 `json:"first_day,omitempty"`
	EveryDays uint16 `json:"every_days,omitempty"`
	Building  string `json:"building,omitempty"`
	Project   string `json:"project,omitempty"`
	Percent   uint16 `json:"percent,omitempty"`
	Text      string `json:"text,omitempty"`
}

// allowedKeys is the JSON keys each part kind may carry besides "kind".
// Anything else in a part is an error, so a typo is never silently dropped.
var allowedKeys = map[PartKind][]string{
	PartForcedTrait:       {"trait", "chance", "who", "n"},
	PartAgeRange:          {"min_age", "max_age"},
	PartForcedCondition:   {"pawn_condition", "chance"},
	PartNoClothes:         {},
	PartStartItem:         {"item", "qty"},
	PartScatterItem:       {"item", "qty", "n", "near"},
	PartStartAnimal:       {"species", "random", "n"},
	PartCondition:         {"condition"},
	PartScheduledIncident: {"incident", "first_day", "every_days"},
	PartDisabledIncident:  {"incident"},
	PartDisallowBuilding:  {"building"},
	PartResearch:          {"project"},
	PartResearchCost:      {"percent"},
	PartIntro:             {"text"},
}

var whoKeys = [...]string{WhoAll: "all", WhoOneRandom: "one_random", WhoNRandom: "n_random"}
var arrivalKeys = [...]string{DropPods: "drop_pods", Standing: "standing", WalkIn: "walk_in"}
var techKeys = [...]string{faction.Neolithic: "neolithic", faction.Industrial: "industrial", faction.Spacer: "spacer"}

// Encode writes d as the versioned JSON file. Fields a part kind does not
// use are not written.
func Encode(d Def) ([]byte, error) {
	if int(d.Arrival) >= len(arrivalKeys) || int(d.Tech) >= len(techKeys) || d.Faction >= faction.NumTypes {
		return nil, fmt.Errorf("scenario: cannot encode unknown arrival, tech or faction type")
	}
	fd := fileDef{
		Key: d.Key, Name: d.Name, Summary: d.Summary, Desc: d.Desc, Seed: d.Seed,
		Faction: faction.DefOf(d.Faction).Key, People: d.People, Choose: d.ChooseFrom,
		Arrival: arrivalKeys[d.Arrival], Tech: techKeys[d.Tech], Hardness: d.Hardness,
		Parts: []json.RawMessage{},
	}
	for i, p := range d.Parts {
		fp, err := encodePart(p)
		if err != nil {
			return nil, fmt.Errorf("scenario: part %d: %w", i+1, err)
		}
		b, err := json.Marshal(fp)
		if err != nil {
			return nil, err
		}
		fd.Parts = append(fd.Parts, b)
	}
	def, err := json.Marshal(fd)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(fileEnvelope{Version: FileVersion, Def: def}, "", "  ")
}

func encodePart(p Part) (filePart, error) {
	k := p.Kind.Key()
	if k == "" {
		return filePart{}, fmt.Errorf("unknown part kind %d", p.Kind)
	}
	fp := filePart{Kind: k}
	switch p.Kind {
	case PartForcedTrait:
		key := trait.DefOf(p.Trait).Key
		if key == "" {
			return fp, fmt.Errorf("unknown trait %d", p.Trait)
		}
		if int(p.Who) >= len(whoKeys) {
			return fp, fmt.Errorf("unknown 'who' %d", p.Who)
		}
		fp.Trait, fp.Chance, fp.Who, fp.N = key, p.Chance, whoKeys[p.Who], p.N
	case PartAgeRange:
		fp.MinAge, fp.MaxAge = p.MinAge, p.MaxAge
	case PartForcedCondition:
		if p.PawnCond == pawngen.NoCondition || int(p.PawnCond) >= len(pawnConditionKeys) {
			return fp, fmt.Errorf("unknown pawn condition %d", p.PawnCond)
		}
		fp.PawnCond, fp.Chance = pawnConditionKeys[p.PawnCond], p.Chance
	case PartStartItem, PartScatterItem:
		if p.Item.String() == "unknown" {
			return fp, fmt.Errorf("unknown item %d", p.Item)
		}
		fp.Item, fp.Qty = p.Item.String(), p.Qty
		if p.Kind == PartScatterItem {
			fp.N, fp.Near = p.N, p.Near
		}
	case PartStartAnimal:
		fp.N, fp.Random = p.N, p.Random
		if !p.Random {
			if p.Species.String() == "animal" {
				return fp, fmt.Errorf("unknown species %d", p.Species)
			}
			fp.Species = p.Species.String()
		}
	case PartCondition:
		if p.Cond.Key() == "" {
			return fp, fmt.Errorf("unknown condition %d", p.Cond)
		}
		fp.Cond = p.Cond.Key()
	case PartScheduledIncident:
		fp.Incident, fp.FirstDay, fp.EveryDays = p.Incident, p.FirstDay, p.EveryDays
	case PartDisabledIncident:
		fp.Incident = p.Incident
	case PartDisallowBuilding:
		if !p.Building.Valid() {
			return fp, fmt.Errorf("unknown building %d", p.Building)
		}
		fp.Building = p.Building.String()
	case PartResearch:
		if p.Project >= research.NumProjects {
			return fp, fmt.Errorf("unknown research project %d", p.Project)
		}
		fp.Project = research.DefOf(p.Project).Name
	case PartResearchCost:
		fp.Percent = p.Percent
	case PartIntro:
		fp.Text = p.Text
	}
	return fp, nil
}

// strict decodes JSON into v, rejecting unknown fields (the error names the
// field).
func strict(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return fmt.Errorf("trailing data after the JSON value")
	}
	return nil
}

// Decode reads a scenario file. Unknown versions, unknown fields, unknown
// part kinds and unknown item, trait, species, building, research,
// condition or enum keys are errors that name what was not understood. The
// result is not validated: run Validate for the rules.
func Decode(b []byte) (Def, error) {
	var env fileEnvelope
	if err := strict(b, &env); err != nil {
		return Def{}, fmt.Errorf("scenario: %w", err)
	}
	if env.Version != FileVersion {
		return Def{}, fmt.Errorf("scenario: unsupported file version %d (this game reads version %d)", env.Version, FileVersion)
	}
	if len(env.Def) == 0 {
		return Def{}, fmt.Errorf("scenario: the file has no \"def\"")
	}
	var fd fileDef
	if err := strict(env.Def, &fd); err != nil {
		return Def{}, fmt.Errorf("scenario: %w", err)
	}
	d := Def{
		Key: fd.Key, Name: fd.Name, Summary: fd.Summary, Desc: fd.Desc, Seed: fd.Seed,
		People: fd.People, ChooseFrom: fd.Choose, Hardness: fd.Hardness,
	}
	var ok bool
	if d.Faction, ok = parseFaction(fd.Faction); !ok {
		return Def{}, fmt.Errorf("scenario: unknown faction type %q", fd.Faction)
	}
	if a, found := indexOf(arrivalKeys[:], fd.Arrival); found {
		d.Arrival = Arrival(a)
	} else {
		return Def{}, fmt.Errorf("scenario: unknown arrival %q", fd.Arrival)
	}
	if t, found := indexOf(techKeys[:], fd.Tech); found {
		d.Tech = faction.Tech(t)
	} else {
		return Def{}, fmt.Errorf("scenario: unknown tech level %q", fd.Tech)
	}
	for i, raw := range fd.Parts {
		p, err := decodePart(raw)
		if err != nil {
			return Def{}, fmt.Errorf("scenario: part %d: %w", i+1, err)
		}
		d.Parts = append(d.Parts, p)
	}
	return d, nil
}

func indexOf(list []string, s string) (int, bool) {
	for i, v := range list {
		if v == s && v != "" {
			return i, true
		}
	}
	return 0, false
}

func parseFaction(key string) (faction.Type, bool) {
	for _, d := range faction.All() {
		if d.Key == key {
			return d.Type, true
		}
	}
	return 0, false
}

func decodePart(raw json.RawMessage) (Part, error) {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		return Part{}, err
	}
	var kindKey string
	if v, ok := keys["kind"]; !ok || json.Unmarshal(v, &kindKey) != nil {
		return Part{}, fmt.Errorf("missing \"kind\"")
	}
	kind, ok := ParsePartKind(kindKey)
	if !ok {
		return Part{}, fmt.Errorf("unknown part kind %q", kindKey)
	}
	allowed := map[string]bool{"kind": true}
	for _, k := range allowedKeys[kind] {
		allowed[k] = true
	}
	var extra []string
	for k := range keys {
		if !allowed[k] {
			extra = append(extra, k)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		return Part{}, fmt.Errorf("%s does not take %v", kindKey, extra)
	}
	var fp filePart
	if err := strict(raw, &fp); err != nil {
		return Part{}, err
	}
	p := Part{Kind: kind}
	switch kind {
	case PartForcedTrait:
		t, found := trait.Parse(fp.Trait)
		if !found {
			return Part{}, fmt.Errorf("unknown trait %q", fp.Trait)
		}
		w := WhoAll
		if fp.Who != "" {
			i, found := indexOf(whoKeys[:], fp.Who)
			if !found {
				return Part{}, fmt.Errorf("unknown 'who' %q", fp.Who)
			}
			w = Who(i)
		}
		p.Trait, p.Chance, p.Who, p.N = t, fp.Chance, w, fp.N
	case PartAgeRange:
		p.MinAge, p.MaxAge = fp.MinAge, fp.MaxAge
	case PartForcedCondition:
		i, found := indexOf(pawnConditionKeys[:], fp.PawnCond)
		if !found {
			return Part{}, fmt.Errorf("unknown pawn condition %q", fp.PawnCond)
		}
		p.PawnCond, p.Chance = pawngen.Condition(i), fp.Chance
	case PartStartItem, PartScatterItem:
		k, found := item.ParseKind(fp.Item)
		if !found {
			return Part{}, fmt.Errorf("unknown item %q", fp.Item)
		}
		p.Item, p.Qty = k, fp.Qty
		if kind == PartScatterItem {
			p.N, p.Near = fp.N, fp.Near
		}
	case PartStartAnimal:
		p.N, p.Random = fp.N, fp.Random
		if !fp.Random {
			s, found := animal.ParseSpecies(fp.Species)
			if !found {
				return Part{}, fmt.Errorf("unknown species %q", fp.Species)
			}
			p.Species = s
		}
	case PartCondition:
		found := false
		for _, c := range Conditions() {
			if c.Key() == fp.Cond {
				p.Cond, found = c, true
			}
		}
		if !found {
			return Part{}, fmt.Errorf("unknown condition %q", fp.Cond)
		}
	case PartScheduledIncident:
		if !validIncident(fp.Incident) {
			return Part{}, fmt.Errorf("unknown incident %q", fp.Incident)
		}
		p.Incident, p.FirstDay, p.EveryDays = fp.Incident, fp.FirstDay, fp.EveryDays
	case PartDisabledIncident:
		if !validIncident(fp.Incident) {
			return Part{}, fmt.Errorf("unknown incident %q", fp.Incident)
		}
		p.Incident = fp.Incident
	case PartDisallowBuilding:
		b, found := construction.Parse(fp.Building)
		if !found {
			return Part{}, fmt.Errorf("unknown building %q", fp.Building)
		}
		p.Building = b
	case PartResearch:
		r, found := research.Parse(fp.Project)
		if !found {
			return Part{}, fmt.Errorf("unknown research project %q", fp.Project)
		}
		p.Project = r
	case PartResearchCost:
		p.Percent = fp.Percent
	case PartIntro:
		p.Text = fp.Text
	}
	return p, nil
}
