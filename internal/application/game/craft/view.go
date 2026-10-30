package craft

import (
	"slices"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/recipe"
	"rimworld-go/internal/domain/sim/entity"
)

// The bills read model (game re-exports these types as game.RecipeView,
// game.BillView, game.BillIngredients and game.BillIngredientView): values only, built from
// the bills and the host's roles, never a reference into the state.

// RecipeView describes a recipe for the bills UI. Bench is its first
// bench; Benches every bench kind that runs it (PRD-3: a recipe can run at
// several, electric twins included), in kind order.
type RecipeView struct {
	ID      uint8
	Name    string
	Label   string
	Bench   string
	Benches []string
	Input   string // e.g. "2 raw food"; several ingredients joined with " + "
	Output  string
}

// RunsAt reports a bench of kind name (construction.Kind.String) runs
// the recipe.
func (r RecipeView) RunsAt(kind string) bool { return slices.Contains(r.Benches, kind) }

// BillView is a bench bill in the read model.
type BillView struct {
	Recipe    string
	Label     string
	Mode      string
	ModeID    uint8
	Target    int
	Done      int
	Suspended bool
	Status    string
	// Waiting: an "until you have" bill resting at its target until the
	// stock falls to ResumeAt (BillHaveEnough status).
	Waiting  bool
	ResumeAt int
	// PRD-1 settings (ConfigureBill, SetBillIngredient): Store is the
	// recipe.StoreMode (0 best stockpile, 1 drop on floor, 2 the stockpile
	// StoreZone), StoreLabel its name; Radius the ingredient search radius
	// from the bench (0: unlimited); SkillMin..SkillMax the allowed skill
	// range (SkillMax 20 when unset); Worker/WorkerName the only colonist
	// allowed (0, "": anyone); UnpauseAt the resume level as set (0: the
	// default; ResumeAt is the one used); Inputs the ingredients; and
	// Ingredients every kind they take, allowed or forbidden.
	Store      uint8
	StoreLabel string
	StoreZone  uint64
	// Source is the storage zone or group the bill takes its ingredients
	// from (RES-1; 0: anywhere in its radius), SourceName its name.
	Source             uint64
	SourceName         string
	Radius             int
	SkillMin, SkillMax uint8
	Worker             uint64
	WorkerName         string
	UnpauseAt          int
	Inputs             string
	Ingredients        BillIngredients
	// The bill now: CurrentWorker is the colonist on it ("" none),
	// Progress the work done in percent once the ingredients are at the
	// bench, Gathering whether they still are being fetched; NoWorker why
	// no colonist may take the bill ("" someone may; NoWorker* strings).
	CurrentWorker string
	Progress      int
	Gathering     bool
	NoWorker      string
	// PRD-2: Stuff is the material the bill is pinned to ("" any the
	// recipe takes and Ingredients allows; StuffKind its item kind, set by
	// ConfigureBill's Stuff), and Quality whether the product rolls a
	// quality from the crafter's skill.
	Stuff     string
	StuffKind uint8
	Quality   bool
}

// BillIngredientView is one ingredient kind a bill may use.
type BillIngredientView struct {
	Kind    uint8 // item.Kind (SetBillIngredient's Stuff)
	Label   string
	Allowed bool
}

// maxIngredients is how many kinds BillIngredients holds (more than any
// recipe's inputs take: raw or dryable food is a dozen or so).
const maxIngredients = 32

// BillIngredients is a bill's ingredient allow list: a fixed-size value
// so BillView stays comparable (the building views are diffed with ==).
type BillIngredients struct {
	n       uint8
	kinds   [maxIngredients]uint8
	allowed uint32
}

// Len is how many ingredient kinds the list holds.
func (l BillIngredients) Len() int { return int(l.n) }

// At is ingredient i (0 <= i < Len).
func (l BillIngredients) At(i int) BillIngredientView {
	k := item.Kind(l.kinds[i])
	return BillIngredientView{Kind: uint8(k), Label: k.String(), Allowed: l.allowed&(1<<i) != 0}
}

// NewBillIngredients builds an allow list from views (Label is derived
// from the kind; past maxIngredients dropped): for tests and fixtures.
func NewBillIngredients(views ...BillIngredientView) BillIngredients {
	var l BillIngredients
	for _, v := range views {
		l.add(item.Kind(v.Kind), v.Allowed)
	}
	return l
}

// add appends kind k (dropped past maxIngredients).
func (l *BillIngredients) add(k item.Kind, allowed bool) {
	if l.n >= maxIngredients {
		return
	}
	l.kinds[l.n] = uint8(k)
	if allowed {
		l.allowed |= 1 << l.n
	}
	l.n++
}

// Why no colonist may take a bill (BillView.NoWorker).
const (
	NoWorkerGone  = "assigned worker is gone"
	NoWorkerSkill = "assigned worker's skill is out of range"
	NoWorkerNone  = "no colonist has the skill"
)

// Recipes lists every recipe but the legacy ones (older saves' recipes,
// upgraded on load).
func Recipes() []RecipeView {
	var out []RecipeView
	for _, d := range recipe.All() {
		if d.Legacy {
			continue
		}
		v := RecipeView{ID: uint8(d.ID), Name: d.Name, Label: d.Label, Bench: d.Bench.String(),
			Input: d.InputsLabel(), Output: d.Output.String()}
		if d.Builds != 0 {
			v.Output = d.Builds.String()
		}
		for _, cd := range construction.All() {
			if cd.Bench && d.At(cd.Kind) {
				v.Benches = append(v.Benches, cd.Name)
			}
		}
		out = append(out, v)
	}
	return out
}

// AppendViews appends bench bid's bill views to out (the game builds
// every bench's into one reused buffer).
func (b *Bills) AppendViews(out []BillView, bid entity.EntityID) []BillView {
	for i := range b.list[bid] {
		bl, st := b.Line(bid, i)
		d := recipe.DefOf(bl.Recipe)
		v := BillView{
			Recipe: d.Name, Label: d.Label, Mode: bl.Mode.String(), ModeID: uint8(bl.Mode),
			Target: bl.Target, Done: bl.Done, Suspended: bl.Suspended, Status: st,
			Waiting: bl.Waiting, ResumeAt: bl.ResumeAt(),
			Store: uint8(bl.Store), StoreLabel: bl.Store.String(), StoreZone: bl.Zone, Radius: bl.Radius,
			SkillMin: bl.SkillMin, SkillMax: bl.MaxSkill(), Worker: uint64(bl.Worker), UnpauseAt: bl.UnpauseAt,
			Inputs: d.InputsLabel(), NoWorker: b.noWorker(bl, d),
			StuffKind: uint8(bl.Stuff), Quality: item.HasQuality(d.Output),
		}
		if bl.Stuff != 0 {
			v.Stuff = bl.Stuff.String()
		}
		if bl.Worker != 0 {
			v.WorkerName = b.h.Name(bl.Worker)
		}
		if v.Source = bl.Source; bl.Source != 0 {
			v.SourceName = b.h.SourceName(bl.Source)
		}
		for k := item.Kind(0); k.String() != "unknown"; k++ { // as d.InputKinds, allocating nothing
			if d.Accepts(k) {
				v.Ingredients.add(k, bl.Allows(k))
			}
		}
		b.current(&v, bid, i, d)
		out = append(out, v)
	}
	return out
}

// current fills v with the colonist working bill i of bench bid, if any,
// and how far along it is.
func (b *Bills) current(v *BillView, bid entity.EntityID, i int, d recipe.Def) {
	for _, pid := range b.h.PawnIDs() {
		j, ok := b.h.Job(pid)
		if !ok || !j.Active() || j.Dest != bid || j.BillIndex != i || j.Kind != job.Bill && j.Kind != job.HaulIngredients {
			continue
		}
		v.CurrentWorker = b.h.Name(pid)
		v.Gathering = j.Kind == job.HaulIngredients || j.Carry == 0
		if !v.Gathering && d.Work > 0 {
			v.Progress = max(0, min(100, 100*(d.Work-j.WorkLeft)/d.Work))
		}
		return
	}
}

// noWorker is why no colonist may take bill bl ("" when one may): its
// assigned worker is gone or outside the skill range, or no colonist has
// the recipe's minimum skill inside the range.
func (b *Bills) noWorker(bl recipe.Bill, d recipe.Def) string {
	h := b.h
	if bl.Worker != 0 {
		if h.Name(bl.Worker) == "" {
			return NoWorkerGone
		}
		if lvl := h.SkillLevel(bl.Worker, d.Learn); lvl < d.MinSkill || !bl.SkillOK(lvl) {
			return NoWorkerSkill
		}
		return ""
	}
	if d.MinSkill == 0 && bl.SkillMin == 0 && bl.SkillMax == 0 {
		return ""
	}
	for _, pid := range h.PawnIDs() {
		if h.Name(pid) == "" {
			continue
		}
		if lvl := h.SkillLevel(pid, d.Learn); lvl >= d.MinSkill && bl.SkillOK(lvl) {
			return ""
		}
	}
	return NoWorkerNone
}
