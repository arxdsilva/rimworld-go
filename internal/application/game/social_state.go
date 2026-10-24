package game

import (
	"cmp"
	"math"
	"slices"

	"rimworld-go/internal/domain/pawn/body"
	"rimworld-go/internal/domain/pawn/look"
	"rimworld-go/internal/domain/pawn/social"
	"rimworld-go/internal/domain/pawn/trait"
	"rimworld-go/internal/domain/sim/entity"
)

// The social web (ENG-003, M3). Edges are sparse: a pair has one only
// once they interacted, remember something of each other or are a couple.
// Each edge holds both directions' opinion memories, the couple state and
// the friend/rival flags (hysteresis), plus a derived opinion cache.
//
// Opinions are a pure function of saved state: traits, the sampled body
// and drug signature (personSig, refreshed at deterministic points), the
// family graph, the pair and the live memories. The cache is valid until
// the next memory expiry and is dropped on every change, so a read model
// that fills it never changes what the simulation sees.

// pairKey is an edge's canonical key (lo < hi).
type pairKey struct{ lo, hi entity.EntityID }

func socKey(a, b entity.EntityID) pairKey {
	if a > b {
		a, b = b, a
	}
	return pairKey{a, b}
}

// edge is one pair's social state; direction 0 is lo's view of hi.
type edge struct {
	lo, hi entity.EntityID
	mem    [2][]social.OpinionMemory
	pair   social.Pair
	friend [2]bool
	rival  [2]bool
	// derived opinion cache (never saved); terms are kept only once a
	// caller asked for them (termsOK)
	ok      [2]bool
	op      [2]int32
	terms   [2][]social.Term
	termsOK [2]bool
	until   [2]uint64
	relVer  [2]uint64
	sigVer  [2][2]uint64
}

// dir is the direction index of from's view.
func (e *edge) dir(from entity.EntityID) int {
	if from == e.lo {
		return 0
	}
	return 1
}

func (e *edge) other(id entity.EntityID) entity.EntityID {
	if id == e.lo {
		return e.hi
	}
	return e.lo
}

func (e *edge) invalidate() { e.ok = [2]bool{} }

// empty reports an edge with nothing left to keep.
func (e *edge) empty() bool {
	return len(e.mem[0]) == 0 && len(e.mem[1]) == 0 && e.pair.Kind == social.PairNone &&
		!e.friend[0] && !e.friend[1] && !e.rival[0] && !e.rival[1]
}

// personSig is the part of a person's body and drug state opinions read,
// sampled at deterministic points (saved).
type personSig struct {
	Disfigured, Blind, Deaf, Addicted bool
	Artificial                        uint8
}

// socialState is the colony's social web (ENG-003, M3-M7): opinion edges,
// couples, gatherings and the social log. nil until the first social
// event; saved as Social.
type socialState struct {
	edges   map[pairKey]*edge                     // lookup only
	adj     map[entity.EntityID][]entity.EntityID // sorted neighbours
	spouses map[entity.EntityID][]social.ID       // derived from pairs, sorted
	sig     map[entity.EntityID]personSig
	next    map[entity.EntityID]uint64 // next tick a pawn may start an interaction
	moodM   map[entity.EntityID]int32  // smoothed mood, milli-points
	logs    map[entity.EntityID][]socialLogEntry
	// relVer bumps on every family or pair change; sigVer per person on a
	// signature change. Both only key the cache (never saved).
	relVer uint64
	sigVer map[entity.EntityID]uint64
	gath   *gatherState // gatherings and lovin' (gathering.go, M5); nil: none
}

func newSocial() *socialState {
	return &socialState{
		edges: map[pairKey]*edge{}, adj: map[entity.EntityID][]entity.EntityID{},
		spouses: map[entity.EntityID][]social.ID{}, sig: map[entity.EntityID]personSig{},
		next: map[entity.EntityID]uint64{}, moodM: map[entity.EntityID]int32{},
		logs: map[entity.EntityID][]socialLogEntry{}, sigVer: map[entity.EntityID]uint64{},
		relVer: 1,
	}
}

// socState is the social state, created on first use.
func (g *Game) socState() *socialState {
	if g.soc == nil {
		g.soc = newSocial()
	}
	return g.soc
}

// edgeOf is the a-b edge (nil: none). Read only.
func (g *Game) edgeOf(a, b entity.EntityID) *edge {
	if g.soc == nil || a == b {
		return nil
	}
	return g.soc.edges[socKey(a, b)]
}

// edgeFor is the a-b edge, created when missing.
func (g *Game) edgeFor(a, b entity.EntityID) *edge {
	s := g.socState()
	k := socKey(a, b)
	if e := s.edges[k]; e != nil {
		return e
	}
	e := &edge{lo: k.lo, hi: k.hi}
	s.edges[k] = e
	s.link(k.lo, k.hi)
	s.link(k.hi, k.lo)
	return e
}

func (s *socialState) link(a, b entity.EntityID) {
	l := s.adj[a]
	if i, found := slices.BinarySearch(l, b); !found {
		s.adj[a] = slices.Insert(l, i, b)
	}
}

func (s *socialState) unlink(a, b entity.EntityID) {
	l := s.adj[a]
	if i, found := slices.BinarySearch(l, b); found {
		l = slices.Delete(l, i, i+1)
		if len(l) == 0 {
			delete(s.adj, a)
		} else {
			s.adj[a] = l
		}
	}
}

func (s *socialState) dropEdge(e *edge) {
	delete(s.edges, pairKey{e.lo, e.hi})
	s.unlink(e.lo, e.hi)
	s.unlink(e.hi, e.lo)
}

// forget drops everything kept about a pruned person (no edges remain).
func (s *socialState) forget(id entity.EntityID) {
	delete(s.sig, id)
	delete(s.sigVer, id)
	delete(s.next, id)
	delete(s.moodM, id)
	delete(s.logs, id)
}

// pairOf is the a-b couple state (zero: none).
func (g *Game) pairOf(a, b entity.EntityID) social.Pair {
	if e := g.edgeOf(a, b); e != nil {
		return e.pair
	}
	return social.Pair{}
}

// setPair sets the a-b couple state (M5 moves couples through it): the
// spouse index and every cached opinion follow.
func (g *Game) setPair(a, b entity.EntityID, p social.Pair) {
	if a == b || a == 0 || b == 0 {
		return
	}
	e := g.edgeFor(a, b)
	was := e.pair.Kind
	e.pair = p
	s := g.soc
	if (was == social.PairSpouse) != (p.Kind == social.PairSpouse) {
		if p.Kind == social.PairSpouse {
			s.spouses[a] = socInsert(s.spouses[a], social.ID(b))
			s.spouses[b] = socInsert(s.spouses[b], social.ID(a))
		} else {
			s.spouses[a] = socRemove(s.spouses[a], social.ID(b))
			s.spouses[b] = socRemove(s.spouses[b], social.ID(a))
		}
	}
	s.relVer++
	if e.empty() {
		s.dropEdge(e)
	}
}

func socInsert[T cmp.Ordered](l []T, x T) []T {
	if i, found := slices.BinarySearch(l, x); !found {
		return slices.Insert(l, i, x)
	}
	return l
}

func socRemove[T cmp.Ordered](l []T, x T) []T {
	if i, found := slices.BinarySearch(l, x); found {
		return slices.Delete(l, i, i+1)
	}
	return l
}

// liveSig samples id's body and drug state.
func (g *Game) liveSig(id entity.EntityID) personSig {
	var s personSig
	if b := g.bodies[id]; b != nil {
		s.Disfigured = b.Disfigured()
		s.Blind = b.Capacity(body.Sight) <= 0
		s.Deaf = b.Capacity(body.Hearing) <= 0
		s.Artificial = uint8(min(b.ArtificialParts(), math.MaxUint8))
	}
	if d := g.drugs[id]; d != nil {
		s.Addicted = len(d.Addictions) > 0
	}
	return s
}

// refreshSig resamples id's signature and drops its cached opinions when
// it changed.
func (g *Game) refreshSig(id entity.EntityID) {
	s := g.socState()
	cur := g.liveSig(id)
	if old, ok := s.sig[id]; ok && old == cur {
		return
	}
	s.sig[id] = cur
	s.sigVer[id]++
}

// invalidateOpinions is for the host systems (body, drugs, traits): id's
// state that opinions read changed, so resample it now.
func (g *Game) invalidateOpinions(id entity.EntityID) {
	if g.personOf(id) == nil {
		return
	}
	g.refreshSig(id)
}

// capacity is id's body capacity c, permille (no body: whole).
func (g *Game) capacity(id entity.EntityID, c body.Capacity) int32 {
	if b := g.bodies[id]; b != nil {
		return b.Capacity(c)
	}
	return 1000
}

// socialPerson is what the opinion and interaction rules read about id.
func (g *Game) socialPerson(id entity.EntityID) social.Person {
	p := social.Person{ID: social.ID(id)}
	if bio, ok := g.personBio(id); ok {
		p.Male = bio.Look.Sex == look.Male
		p.AgeYears = bio.AgeYears()
		p.Traits = bio.Traits
		var beauty int
		for _, t := range bio.Traits {
			beauty += int(trait.DefOf(t).Beauty)
		}
		p.Beauty = int8(beauty)
	}
	var sig personSig
	if g.soc != nil {
		sig = g.soc.sig[id]
	}
	p.Disfigured, p.Blind, p.Deaf, p.Addicted = sig.Disfigured, sig.Blind, sig.Deaf, sig.Addicted
	p.ArtificialParts = int(sig.Artificial)
	if pr := g.personOf(id); pr != nil && pr.Where == whereDead {
		p.Dead = true
	}
	return p
}

// opinion is from's opinion of to, centi-points.
func (g *Game) opinion(from, to entity.EntityID) int32 {
	v, _ := g.opinionTerms(from, to, false)
	return v
}

// opinionTerms is from's opinion of to and (withTerms) its breakdown. An
// edge caches it until its next memory expiry.
func (g *Game) opinionTerms(from, to entity.EntityID, withTerms bool) (int32, []social.Term) {
	if from == to || from == 0 || to == 0 {
		return 0, nil
	}
	now := uint64(g.clock.Now())
	e := g.edgeOf(from, to)
	var s *socialState
	if g.soc != nil {
		s = g.soc
	}
	cached := false
	if e != nil {
		d := e.dir(from)
		if e.ok[d] && now < e.until[d] && e.relVer[d] == s.relVer && e.sigVer[d] == [2]uint64{s.sigVer[from], s.sigVer[to]} {
			if !withTerms || e.termsOK[d] {
				return e.op[d], e.terms[d]
			}
			cached = true // the value holds; only its breakdown is missing
		}
	}
	in := social.Inputs{Observer: g.socialPerson(from), Target: g.socialPerson(to), Relations: g.kinOf(from, to), Now: now}
	until := uint64(math.MaxUint64)
	if e != nil {
		d := e.dir(from)
		in.Memories = e.mem[d]
		for _, m := range e.mem[d] {
			if m.Expires > now && m.Expires < until {
				until = m.Expires
			}
		}
	}
	if cached {
		// a view's breakdown never changes the cached value the
		// simulation reads
		d := e.dir(from)
		_, terms := social.Opinion(in)
		e.terms[d], e.termsOK[d] = terms, true
		return e.op[d], terms
	}
	var v int32
	var terms []social.Term
	if withTerms {
		v, terms = social.Opinion(in)
	} else {
		v = social.OpinionValue(in) // the simulation's reads: no breakdown built
	}
	if e != nil && s != nil {
		d := e.dir(from)
		_, okFrom := s.sig[from]
		_, okTo := s.sig[to]
		if okFrom && okTo {
			e.ok[d], e.op[d], e.terms[d], e.termsOK[d], e.until[d] = true, v, terms, withTerms, until
			e.relVer[d], e.sigVer[d] = s.relVer, [2]uint64{s.sigVer[from], s.sigVer[to]}
		}
	}
	return v, terms
}

// addOpinionMemory makes from remember k about subject (who it concerns).
func (g *Game) addOpinionMemory(from, subject entity.EntityID, k social.MemKind, now uint64) {
	g.addScaledOpinionMemory(from, subject, k, now, 0)
}

// hasOpinionMemory reports from holds a live memory k about subject.
func (g *Game) hasOpinionMemory(from, subject entity.EntityID, k social.MemKind, now uint64) bool {
	e := g.edgeOf(from, subject)
	return e != nil && social.HasMemory(e.mem[e.dir(from)], k, social.ID(subject), now)
}

// addScaledOpinionMemory is addOpinionMemory landing at scale (permille
// of the memory's value, 0: full; social.ImpactScale).
func (g *Game) addScaledOpinionMemory(from, subject entity.EntityID, k social.MemKind, now uint64, scale int16) {
	if from == subject || from == 0 || subject == 0 {
		return
	}
	e := g.edgeFor(from, subject)
	d := e.dir(from)
	e.mem[d] = social.AddScaledMemory(e.mem[d], k, social.ID(subject), now, scale)
	e.invalidate()
	g.updateFlags(e, d)
}

// updateFlags moves direction d's friend and rival flags with hysteresis:
// friend from >= FriendEnter until below FriendExit, rival from <=
// RivalEnter until above RivalExit.
func (g *Game) updateFlags(e *edge, d int) {
	from, to := e.lo, e.hi
	if d == 1 {
		from, to = to, from
	}
	o := g.opinion(from, to)
	e.friend[d] = friendBand(e.friend[d], o)
	e.rival[d] = rivalBand(e.rival[d], o)
}

func friendBand(was bool, o int32) bool {
	if was {
		return o >= social.FriendExit
	}
	return o >= social.FriendEnter
}

func rivalBand(was bool, o int32) bool {
	if was {
		return o <= social.RivalExit
	}
	return o <= social.RivalEnter
}

// friendOf and rivalOf read from's flags toward to.
func (g *Game) friendOf(from, to entity.EntityID) bool {
	if e := g.edgeOf(from, to); e != nil {
		return e.friend[e.dir(from)]
	}
	return false
}

func (g *Game) rivalOf(from, to entity.EntityID) bool {
	if e := g.edgeOf(from, to); e != nil {
		return e.rival[e.dir(from)]
	}
	return false
}

// tendEdges is id's periodic edge upkeep: expired memories pruned, its
// flags toward each neighbour moved, emptied edges dropped.
func (g *Game) tendEdges(id entity.EntityID, now uint64) {
	s := g.soc
	if s == nil {
		return
	}
	adj := s.adj[id]
	for i := 0; i < len(adj); i++ {
		e := s.edges[socKey(id, adj[i])]
		if e == nil {
			continue
		}
		g.pruneEdge(e, now)
		g.updateFlags(e, e.dir(id))
		if e.empty() {
			s.dropEdge(e)
			adj = s.adj[id]
			i--
		}
	}
}

// pruneEdge drops expired memories (a value-preserving cleanup).
func (g *Game) pruneEdge(e *edge, now uint64) {
	for d := range e.mem {
		if len(e.mem[d]) > 0 {
			e.mem[d] = social.PruneMemories(e.mem[d], now)
			if len(e.mem[d]) == 0 {
				e.mem[d] = nil
			}
		}
	}
}

// socialDay is the daily upkeep: every edge pruned, emptied ones dropped,
// strangers forgotten.
func (g *Game) socialDay(now uint64) {
	s := g.soc
	if s == nil {
		return
	}
	keys := make([]pairKey, 0, len(s.edges))
	for k := range s.edges {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, cmpKey)
	for _, k := range keys {
		e := s.edges[k]
		g.pruneEdge(e, now)
		if e.empty() {
			s.dropEdge(e)
		}
	}
	g.pruneStrangers()
}

func cmpKey(a, b pairKey) int {
	switch {
	case a.lo != b.lo:
		if a.lo < b.lo {
			return -1
		}
		return 1
	case a.hi < b.hi:
		return -1
	case a.hi > b.hi:
		return 1
	}
	return 0
}

// socialSave is socialState as saved.
type socialSave struct {
	Version uint8 // format of this block; 0 = empty
	Edges   []edgeSave
	Sigs    []sigSave
	Next    []tickSave
	Mood    []moodSave
	Logs    []logSave
	Gather  *gatherSave // M5: gatherings, retries, lovin' cooldowns (nil: none)
}

type edgeSave struct {
	A, B         entity.EntityID // A < B
	MemAB, MemBA []social.OpinionMemory
	Pair         social.Pair
	Flags        uint8 // friendAB, friendBA, rivalAB, rivalBA bits
}

type sigSave struct {
	ID  entity.EntityID
	Sig personSig
}

type tickSave struct {
	ID   entity.EntityID
	Tick uint64
}

type moodSave struct {
	ID    entity.EntityID
	Milli int32
}

type logSave struct {
	ID      entity.EntityID
	Entries []socialLogEntry
}

func sortedIDs[V any](m map[entity.EntityID]V) []entity.EntityID {
	out := make([]entity.EntityID, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

// exportSocial is the Social save field (nil: nothing yet).
func (g *Game) exportSocial() *socialSave {
	s := g.soc
	if s == nil {
		return nil
	}
	out := &socialSave{Version: 1}
	keys := make([]pairKey, 0, len(s.edges))
	for k := range s.edges {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, cmpKey)
	for _, k := range keys {
		e := s.edges[k]
		es := edgeSave{A: e.lo, B: e.hi, MemAB: slices.Clone(e.mem[0]), MemBA: slices.Clone(e.mem[1]), Pair: e.pair}
		for i, f := range [4]bool{e.friend[0], e.friend[1], e.rival[0], e.rival[1]} {
			if f {
				es.Flags |= 1 << i
			}
		}
		out.Edges = append(out.Edges, es)
	}
	for _, id := range sortedIDs(s.sig) {
		out.Sigs = append(out.Sigs, sigSave{id, s.sig[id]})
	}
	for _, id := range sortedIDs(s.next) {
		out.Next = append(out.Next, tickSave{id, s.next[id]})
	}
	for _, id := range sortedIDs(s.moodM) {
		out.Mood = append(out.Mood, moodSave{id, s.moodM[id]})
	}
	for _, id := range sortedIDs(s.logs) {
		if l := s.logs[id]; len(l) > 0 {
			out.Logs = append(out.Logs, logSave{id, slices.Clone(l)})
		}
	}
	out.Gather = s.gath.export() // gathering.go
	if len(out.Edges) == 0 && len(out.Sigs) == 0 && len(out.Next) == 0 && len(out.Mood) == 0 && len(out.Logs) == 0 && out.Gather == nil {
		return nil
	}
	return out
}

// importSocial restores exportSocial's state (nil: fresh).
func (g *Game) importSocial(in *socialSave) {
	g.soc = nil
	if in == nil {
		return
	}
	s := g.socState()
	for _, es := range in.Edges {
		e := g.edgeFor(es.A, es.B)
		e.mem[0], e.mem[1] = es.MemAB, es.MemBA
		if len(e.mem[0]) == 0 {
			e.mem[0] = nil
		}
		if len(e.mem[1]) == 0 {
			e.mem[1] = nil
		}
		e.pair = es.Pair
		e.friend = [2]bool{es.Flags&1 != 0, es.Flags&2 != 0}
		e.rival = [2]bool{es.Flags&4 != 0, es.Flags&8 != 0}
		if e.pair.Kind == social.PairSpouse {
			s.spouses[e.lo] = socInsert(s.spouses[e.lo], social.ID(e.hi))
			s.spouses[e.hi] = socInsert(s.spouses[e.hi], social.ID(e.lo))
		}
	}
	for _, x := range in.Sigs {
		s.sig[x.ID] = x.Sig
	}
	for _, x := range in.Next {
		s.next[x.ID] = x.Tick
	}
	for _, x := range in.Mood {
		s.moodM[x.ID] = x.Milli
	}
	for _, x := range in.Logs {
		s.logs[x.ID] = x.Entries
	}
	g.importGather(in.Gather) // gathering.go
}
