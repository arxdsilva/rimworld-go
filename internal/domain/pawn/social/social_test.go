package social

import (
	"reflect"
	"testing"

	"rimworld-go/internal/domain/pawn/mood"
	"rimworld-go/internal/domain/pawn/trait"
)

type fam struct {
	par map[ID][2]ID
	sp  map[ID][]ID
}

func (f fam) Parents(id ID) [2]ID { return f.par[id] }
func (f fam) Spouses(id ID) []ID  { return f.sp[id] }

func has(ks []Kind, k Kind) bool {
	for _, x := range ks {
		if x == k {
			return true
		}
	}
	return false
}

// Sample family. Mothers/fathers: GGP=1,2 -> GP=3(f) ; 3 and 4(w) -> P=5(f),
// 5 and 6 -> me=10, sib=11, 12 (half: 5 + 7). Uncle 8 (child of 3,4),
// cousin 9 (child of 8), grandparent 3, great-grandparent 1.
func sample() fam {
	return fam{
		par: map[ID][2]ID{
			3:  {2, 1}, // GP child of GGP 2,1
			5:  {4, 3}, // parent 5 child of 4,3
			8:  {4, 3}, // uncle 8
			9:  {20, 8},
			10: {5, 6},
			11: {5, 6},
			12: {5, 7}, // half sibling
			13: {0, 8},
			14: {0, 9}, // child of cousin 9
			30: {5, 0}, // dummy
			40: {50, 0},
			// second cousin: me=10, grandparents 3,4; cousin 9's child 14 is 1st cousin once removed;
			// second cousin 61: parent 60 is child of 59, who is child of 3,4's parent? build below
			59: {2, 1}, // sibling of 3
			60: {0, 59},
			61: {0, 60},
			// grand-uncle of 10: 59 (sibling of 3, grandparent of 10).
			70: {0, 11}, // child of sibling 11 -> nephew of 10
			71: {0, 70}, // grand nephew
			80: {0, 10}, // child of me
			81: {0, 80}, // grandchild
			82: {0, 81}, // great-grandchild
			// step: 6 has spouse 90, who is stepparent to 10's... 90 spouse of 5
			// in-law
			100: {0, 0},
			101: {5, 6},
			102: {0, 0},
		},
		sp: map[ID][]ID{
			5: {6, 90}, 6: {5}, 90: {5},
			10: {100}, 100: {10},
			80: {110}, 110: {80},
		},
	}
}

func TestKinOf(t *testing.T) {
	g := sample()
	g.par[100] = [2]ID{103, 0}
	g.par[111] = [2]ID{0, 0}
	tests := []struct {
		name string
		a, b ID
		want Kind
	}{
		{"parent", 10, 5, Parent},
		{"parent father", 10, 6, Parent},
		{"child", 5, 10, Child},
		{"sibling", 10, 11, Sibling},
		{"half sibling", 10, 12, HalfSibling},
		{"grandparent", 10, 3, Grandparent},
		{"grandchild", 3, 10, Grandchild},
		{"great-grandparent", 10, 1, GreatGrandparent},
		{"great-grandchild", 1, 10, GreatGrandchild},
		{"aunt/uncle", 10, 8, AuntUncle},
		{"niece/nephew", 8, 10, NieceNephew},
		{"cousin", 10, 9, Cousin},
		{"grand-aunt/uncle", 10, 59, GrandAuntUncle},
		{"grand-niece/nephew", 59, 10, GrandNieceNephew},
		{"second cousin", 10, 61, SecondCousin},
		{"nephew via sibling", 10, 70, NieceNephew},
		{"stepparent", 10, 90, StepParent},
		{"stepchild", 90, 10, StepChild},
		{"parent-in-law", 10, 103, ParentInLaw},
		{"child-in-law", 80, 100, ChildInLaw},
	}
	g.par[100] = [2]ID{103, 0}
	g.sp[80] = []ID{110}
	g.par[110] = [2]ID{0, 0}
	g.sp[10] = []ID{100}
	g.sp[100] = []ID{10}
	// child-in-law: 10's child 80, 80's spouse 110.
	tests[len(tests)-1] = struct {
		name string
		a, b ID
		want Kind
	}{"child-in-law", 10, 110, ChildInLaw}
	for _, tc := range tests {
		got := KinOf(tc.a, tc.b, g)
		if !has(got, tc.want) {
			t.Errorf("%s: KinOf(%d,%d)=%v want %v", tc.name, tc.a, tc.b, got, tc.want)
		}
	}
}

func TestKinOfEdges(t *testing.T) {
	g := sample()
	if KinOf(10, 10, g) != nil || KinOf(0, 1, g) != nil || KinOf(1, 2, nil) != nil {
		t.Fatal("expected nil")
	}
	if len(KinOf(10, 999, g)) != 0 {
		t.Fatal("strangers have no kin")
	}
	// Beyond depth 4 -> Kin.
	deep := fam{par: map[ID][2]ID{1: {0, 2}, 2: {0, 3}, 3: {0, 4}, 4: {0, 5}, 5: {0, 6}}}
	if got := KinOf(1, 5, deep); !has(got, Kin) {
		t.Fatalf("deep ancestor: %v", got)
	}
	if got := KinOf(5, 1, deep); !has(got, Kin) {
		t.Fatalf("deep descendant: %v", got)
	}
	// Distant common ancestor -> Kin.
	d2 := fam{par: map[ID][2]ID{1: {0, 2}, 2: {0, 3}, 3: {0, 9}, 4: {0, 5}, 5: {0, 6}, 6: {0, 7}, 7: {0, 9}}}
	if got := KinOf(1, 4, d2); !has(got, Kin) {
		t.Fatalf("distant: %v", got)
	}
}

func TestStrongestKin(t *testing.T) {
	if _, ok := StrongestKin(nil); ok {
		t.Fatal("empty")
	}
	k, _ := StrongestKin([]Kind{Cousin, Parent, Kin})
	if k != Parent {
		t.Fatalf("got %v", k)
	}
	k, _ = StrongestKin([]Kind{Child, Parent}) // equal bonus: lowest kind
	if k != Parent {
		t.Fatalf("tie: %v", k)
	}
}

func TestLabelsAndBonus(t *testing.T) {
	tests := []struct {
		k    Kind
		male bool
		want string
	}{
		{Parent, true, "father"}, {Parent, false, "mother"}, {Child, true, "son"}, {Child, false, "daughter"},
		{Sibling, true, "brother"}, {Sibling, false, "sister"}, {Spouse, true, "husband"}, {Spouse, false, "wife"},
		{Fiance, true, "fiancé"}, {Fiance, false, "fiancée"}, {ExLover, true, "ex-lover"}, {ExSpouse, true, "ex-husband"},
		{Cousin, true, "cousin"}, {AuntUncle, false, "aunt"}, {AuntUncle, true, "uncle"},
	}
	for _, tc := range tests {
		if got := Label(tc.k, tc.male); got != tc.want {
			t.Errorf("Label(%v,%v)=%q want %q", tc.k, tc.male, got, tc.want)
		}
	}
	for k := Kind(0); k < NumKinds; k++ {
		if Label(k, true) == "" || Label(k, false) == "" {
			t.Errorf("kind %d unlabeled", k)
		}
	}
	if Label(NumKinds, true) != "" || Bonus(NumKinds) != 0 {
		t.Fatal("out of range")
	}
	if Bonus(Fiance) != 3000 || Bonus(ExLover) != -1500 || Bonus(ExSpouse) != -1500 {
		t.Fatal("romance bonus")
	}
	if !IsRomantic(Lover) || IsRomantic(Parent) {
		t.Fatal("IsRomantic")
	}
}

func TestCurve(t *testing.T) {
	c := Curve{{0, 0}, {10, 100}, {20, 100}}
	for x, want := range map[int32]int32{-5: 0, 0: 0, 5: 50, 10: 100, 15: 100, 99: 100} {
		if got := c.Eval(x); got != want {
			t.Errorf("Eval(%d)=%d want %d", x, got, want)
		}
	}
	if (Curve{}).Eval(3) != 0 {
		t.Fatal("empty")
	}
	if got := (Curve{{1, 5}, {1, 7}}).Eval(1); got != 5 {
		t.Fatalf("vertical: %d", got)
	}
	if got := (Curve{{0, 1}, {1, 5}, {1, 9}}).Eval(2); got != 9 {
		t.Fatalf("flat tail: %d", got)
	}
	if got := (Curve{{0, 1}, {1, 5}, {1, 9}}).Eval(1); got != 5 {
		t.Fatalf("zero span: %d", got)
	}
}

func TestCurveEndpoints(t *testing.T) {
	for x, want := range map[int32]int32{-10000: 6000, 0: 1000, 5000: 100, 10000: 0, -20000: 6000} {
		if got := OpinionNegCurve.Eval(x); got != want {
			t.Errorf("opinion curve %d=%d want %d", x, got, want)
		}
	}
	if DeepTalkCompatCurve.Eval(-5000) != 0 || NegativeCompatCurve.Eval(0) != 1000 {
		t.Fatal("compat curves")
	}
	if ProposalAccept(-100) != 0 || ProposalAccept(20000) != 1000 {
		t.Fatal("accept")
	}
}

func TestCompatibility(t *testing.T) {
	for a := ID(1); a < 40; a++ {
		for b := ID(1); b < 40; b++ {
			if Compatibility(7, a, b, 30, 35) != Compatibility(7, b, a, 35, 30) {
				t.Fatalf("not canonical %d %d", a, b)
			}
		}
	}
	if Compatibility(7, 1, 2, 30, 30) != Compatibility(7, 1, 2, 30, 30) {
		t.Fatal("unstable")
	}
	if Compatibility(7, 1, 2, 30, 30) == Compatibility(8, 1, 2, 30, 30) &&
		Compatibility(7, 1, 3, 30, 30) == Compatibility(8, 1, 3, 30, 30) {
		t.Fatal("seed ignored")
	}
	// Spread: sigma about 1000.
	var sum, sq int64
	n := int64(0)
	for a := ID(1); a <= 200; a++ {
		for b := a + 1; b <= 200; b++ {
			v := int64(Compatibility(3, a, b, 25, 25)) - 450
			sum += v
			sq += v * v
			n++
		}
	}
	mean := sum / n
	varr := sq/n - mean*mean
	if mean < -60 || mean > 60 || varr < 900*900 || varr > 1100*1100 {
		t.Fatalf("distribution mean=%d var=%d", mean, varr)
	}
}

func TestAgeTerm(t *testing.T) {
	for _, tc := range []struct {
		a, b int
		want int32
	}{{30, 30, 450}, {30, 35, 225}, {30, 40, 0}, {30, 50, -450}, {10, 80, -450}, {50, 30, -450}} {
		if got := ageTerm(tc.a, tc.b); got != tc.want {
			t.Errorf("ageTerm(%d,%d)=%d want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestMemoryStackingAndPrune(t *testing.T) {
	var ms []OpinionMemory
	for i := 0; i < 3; i++ {
		ms = AddMemory(ms, MemInsulted, 5, 100)
	}
	// limit 5, factor 900: 3 copies
	got := Sum(ms, 5, 100)
	want := int32(-1500 - 1350 - 1215)
	if got != want {
		t.Fatalf("stacked %d want %d", got, want)
	}
	if Sum(ms, 6, 100) != 0 {
		t.Fatal("other subject")
	}
	for i := 0; i < 10; i++ {
		ms = AddMemory(ms, MemInsulted, 5, 200+uint64(i))
	}
	if len(ms) != 5 {
		t.Fatalf("limit: %d copies", len(ms))
	}
	// Limit 1: refresh.
	var rs []OpinionMemory
	rs = AddMemory(rs, MemRebuffedMe, 1, 0)
	rs = AddMemory(rs, MemRebuffedMe, 1, 50)
	if len(rs) != 1 || rs[0].Expires != 50+3*Day {
		t.Fatalf("refresh: %+v", rs)
	}
	if !HasMemory(rs, MemRebuffedMe, 1, 60) || HasMemory(rs, MemRebuffedMe, 1, 50+3*Day) || HasMemory(rs, MemRebuffedMe, 2, 60) {
		t.Fatal("HasMemory")
	}
	rs = PruneMemories(rs, 50+3*Day)
	if len(rs) != 0 {
		t.Fatal("prune")
	}
	if got := AddMemory(nil, NumMemKinds, 1, 0); got != nil {
		t.Fatal("unknown kind")
	}
	if MemDef(NumMemKinds).Duration != 0 {
		t.Fatal("MemDef")
	}
	// Expired copies do not count toward the limit.
	var es []OpinionMemory
	es = AddMemory(es, MemRebuffedMe, 1, 0)
	es = AddMemory(es, MemRebuffedMe, 1, 10*Day)
	if len(es) != 2 {
		t.Fatal("expired should add")
	}
	if Sum(es, 1, 10*Day) != memRebuffedMe {
		t.Fatalf("expired excluded: %d", Sum(es, 1, 10*Day))
	}
}

func TestChitchatCap(t *testing.T) {
	var ms []OpinionMemory
	for i := 0; i < 50; i++ {
		ms = AddMemory(ms, MemChitchat, 2, 0)
	}
	if s := Sum(ms, 2, 0); s <= 0 || s > ChitchatCap {
		t.Fatalf("chitchat %d", s)
	}
	// Forced cap path.
	big := make([]OpinionMemory, 0, 40)
	for i := 0; i < 40; i++ {
		big = append(big, OpinionMemory{Kind: MemChitchat, Subject: 2, Expires: 100})
	}
	d := memDefs[MemChitchat]
	memDefs[MemChitchat].StackFactor = 1000
	defer func() { memDefs[MemChitchat] = d }()
	if Sum(big, 2, 0) != ChitchatCap {
		t.Fatalf("cap %d", Sum(big, 2, 0))
	}
}

func p(id ID, male bool, tr ...trait.ID) Person {
	return Person{ID: id, Male: male, AgeYears: 30, Traits: tr}
}

func opin(o, tg Person, rel ...Kind) int32 {
	v, _ := Opinion(Inputs{Observer: o, Target: tg, Relations: rel})
	return v
}

func TestOpinionTerms(t *testing.T) {
	type tc struct {
		name string
		o, t Person
		rel  []Kind
		want int32
	}
	beauty := func(b int8) Person { x := p(2, true); x.Beauty = b; return x }
	dis := p(2, true)
	dis.Disfigured = true
	disBeautiful := beauty(2)
	disBeautiful.Disfigured = true
	art := func(n int) Person { x := p(2, true); x.ArtificialParts = n; return x }
	add := p(2, true)
	add.Addicted = true
	blind := p(1, true)
	blind.Blind = true
	deaf := p(1, true, trait.GratingVoice)
	deaf.Deaf = true
	tests := []tc{
		{"none", p(1, true), p(2, true), nil, 0},
		{"strongest kin", p(1, true), p(2, true), []Kind{Cousin, Parent, Kin}, 3000},
		{"kin plus spouse", p(1, true), p(2, true), []Kind{Sibling, Spouse}, 2000 + 3000},
		{"ex lover", p(1, true), p(2, true), []Kind{ExLover}, -1500},
		{"fiance", p(1, true), p(2, true), []Kind{Fiance}, 3000},
		{"beauty 1", p(1, true), beauty(1), nil, 2000},
		{"beauty 2", p(1, true), beauty(2), nil, 4000},
		{"beauty -1", p(1, true), beauty(-1), nil, -2000},
		{"beauty -2", p(1, true), beauty(-2), nil, -4000},
		{"gracious ignores beauty", p(1, true, trait.Gracious), beauty(-2), nil, 0},
		{"austere ignores beauty", p(1, true, trait.Austere), beauty(2), nil, 0},
		{"blind ignores beauty", blind, beauty(2), nil, 0},
		{"disfigured", p(1, true), dis, nil, -1500},
		{"disfigured cancels beauty", p(1, true), disBeautiful, nil, -1500},
		{"gracious ignores disfigured", p(1, true, trait.Gracious), dis, nil, 0},
		{"grating voice", p(1, true), p(2, true, trait.GratingVoice), nil, -2500},
		{"rasping", p(1, true), p(2, true, trait.RaspingBreath), nil, -2500},
		{"both voices", p(1, true), p(2, true, trait.RaspingBreath, trait.GratingVoice), nil, -5000},
		{"deaf observer", deaf, p(2, true, trait.GratingVoice), nil, 0},
		{"gracious voice", p(1, true, trait.Gracious), p(2, true, trait.GratingVoice), nil, 0},
		{"distrusts men", p(1, true, trait.DistrustsMen), p(2, true), nil, -2500},
		{"distrusts men vs woman", p(1, true, trait.DistrustsMen), p(2, false), nil, 0},
		{"distrusts women", p(1, true, trait.DistrustsWomen), p(2, false), nil, -2500},
		{"distrusts women vs man", p(1, true, trait.DistrustsWomen), p(2, true), nil, 0},
		{"tireless unhurried", p(1, true, trait.Tireless), p(2, true, trait.Unhurried), nil, -2000},
		{"tireless idle", p(1, true, trait.Tireless), p(2, true, trait.Idle), nil, -3000},
		{"abstainer addicted", p(1, true, trait.Abstainer), add, nil, -2500},
		{"abstainer indulgent", p(1, true, trait.Abstainer), p(2, true, trait.Indulgent), nil, -2000},
		{"abstainer hedonist", p(1, true, trait.Abstainer), p(2, true, trait.Hedonist), nil, -3000},
		{"augment fan", p(1, true, trait.AugmentFan), art(2), nil, 1600},
		{"augment fan cap", p(1, true, trait.AugmentFan), art(9), nil, 4000},
		{"purist", p(1, true, trait.NaturalPurist), art(2), nil, -1600},
		{"purist cap", p(1, true, trait.NaturalPurist), art(9), nil, -4000},
		{"clamp high", p(1, true), p(2, true), []Kind{Spouse, Parent}, 3000 + 3000},
	}
	for _, c := range tests {
		if got := opin(c.o, c.t, c.rel...); got != c.want {
			t.Errorf("%s: %d want %d", c.name, got, c.want)
		}
	}
}

func TestOpinionClampAndAsymmetry(t *testing.T) {
	a, b := p(1, true, trait.Abstainer, trait.Tireless), p(2, true, trait.Hedonist, trait.Idle, trait.GratingVoice)
	b.Beauty = -2
	b.Disfigured = true
	v := opin(a, b)
	if v != -10000 {
		t.Fatalf("clamp low: %d", v)
	}
	var ms []OpinionMemory
	for i := 0; i < 50; i++ {
		ms = AddMemory(ms, MemKindWords, 2, 0)
	}
	hi := p(3, true)
	got, terms := Opinion(Inputs{Observer: hi, Target: p(2, true), Relations: []Kind{Spouse}, Memories: ms, Now: 0})
	if got > 10000 || len(terms) < 2 {
		t.Fatalf("got %d terms %v", got, terms)
	}
	if opin(a, b) == opin(b, a) {
		t.Fatal("expected asymmetry")
	}
}

func TestOpinionMemoriesAndTerms(t *testing.T) {
	ms := AddMemory(nil, MemCheatedOnMe, 2, 0)
	v, terms := Opinion(Inputs{Observer: p(1, true), Target: p(2, true), Memories: ms, Now: 10})
	if v != -7000 || len(terms) != 1 || terms[0].Label == "" {
		t.Fatalf("%d %v", v, terms)
	}
	v, _ = Opinion(Inputs{Observer: p(1, true), Target: p(2, true), Memories: ms, Now: 26 * Day})
	if v != 0 {
		t.Fatal("expired")
	}
	v, _ = Opinion(Inputs{Observer: p(1, true), Target: p(3, true), Memories: ms, Now: 10})
	if v != 0 {
		t.Fatal("other subject")
	}
}

func fixedRoll(v int) func(int) int {
	return func(n int) int {
		if v >= n {
			return n - 1
		}
		return v
	}
}

func TestWeightsAndChoose(t *testing.T) {
	c := InteractCtx{Initiator: p(1, true), Recipient: p(2, false), Compat: 0}
	w := Weights(c)
	if w[Chitchat] != 1000 || w[DeepTalk] != 37 || w[Slight] != 20 || w[Insult] != 7 || w[KindWords] != 0 {
		t.Fatalf("weights %v", w)
	}
	// Gracious: kind words only, no slight/insult.
	c.Initiator = p(1, true, trait.Gracious)
	w = Weights(c)
	if w[KindWords] != 10 || w[Slight] != 0 || w[Insult] != 0 {
		t.Fatalf("gracious %v", w)
	}
	// ColdHearted never kind words.
	c.Initiator = p(1, true, trait.Gracious, trait.ColdHearted)
	if Weights(c)[KindWords] != 0 {
		t.Fatal("cold-hearted kind words")
	}
	// High opinion kills insults; low opinion boosts them.
	c.Initiator = p(1, true)
	c.OpinionIR = 10000
	if Weights(c)[Insult] != 0 {
		t.Fatal("insult at +100")
	}
	c.OpinionIR = -10000
	if Weights(c)[Insult] != 42 {
		t.Fatalf("insult at -100: %d", Weights(c)[Insult])
	}
	// Romance.
	c = InteractCtx{Initiator: p(1, true), Attraction: 1000}
	if Weights(c)[RomanceAttempt] != 100 {
		t.Fatal("romance weight")
	}
	c.Partnered = true
	if Weights(c)[RomanceAttempt] != 25 {
		t.Fatal("affair weight")
	}
	c.Pair = PairLover
	c.OpinionIR = -5000
	w = Weights(c)
	if w[RomanceAttempt] != 0 || w[Breakup] == 0 || w[Proposal] != 0 {
		t.Fatalf("lover low opinion %v", w)
	}
	c.OpinionIR = 8000
	if Weights(c)[Proposal] == 0 {
		t.Fatal("proposal")
	}
	c.Pair = PairSpouse
	if Weights(c)[Proposal] != 0 {
		t.Fatal("spouse proposes")
	}
	// Choose: deterministic and covers ranges.
	c = InteractCtx{Initiator: p(1, true)}
	if Choose(fixedRoll(0), c) != Chitchat {
		t.Fatal("first")
	}
	if Choose(fixedRoll(1<<30), c) == Chitchat {
		t.Fatal("last should be a rare interaction")
	}
	c.Initiator = p(1, true)
	c.OpinionIR = 10000
	c.Compat = -2000
	if Choose(fixedRoll(1<<30), c) != DeepTalk && Choose(fixedRoll(1<<30), c) != Chitchat {
		t.Fatal("unexpected")
	}
	seen := map[Interaction]bool{}
	for r := 0; r < 1200; r++ {
		cc := InteractCtx{Initiator: p(1, true, trait.Gracious), Compat: 1000, Attraction: 800}
		seen[Choose(fixedRoll(r), cc)] = true
	}
	if !seen[Chitchat] || !seen[DeepTalk] || !seen[KindWords] || !seen[RomanceAttempt] {
		t.Fatalf("coverage %v", seen)
	}
	zero := func(int) int { return 0 }
	if Choose(zero, InteractCtx{}) != Chitchat {
		t.Fatal("zero")
	}
	// Total zero path.
	if got := Choose(zero, InteractCtx{Initiator: p(1, true, trait.Gracious, trait.ColdHearted)}); got != Chitchat {
		t.Fatal(got)
	}
}

func TestWeightsAsymmetric(t *testing.T) {
	a := InteractCtx{Initiator: p(1, true), OpinionIR: -8000, Compat: -500}
	b := InteractCtx{Initiator: p(2, true), OpinionIR: 8000, Compat: -500}
	if Weights(a)[Insult] <= Weights(b)[Insult] {
		t.Fatal("A->B must differ from B->A")
	}
}

func TestEffectsAndImpact(t *testing.T) {
	if e := Effects(Insult); !e.HasMood || e.Mood != mood.Insulted || e.Memory != MemInsulted {
		t.Fatal("insult")
	}
	if e := Effects(Slight); e.HasMood || e.Memory != MemSlighted {
		t.Fatal("slight")
	}
	if e := Effects(KindWords); e.Mood != mood.KindWords || e.Memory != MemKindWords {
		t.Fatal("kind")
	}
	if e := Effects(Chitchat); e.HasMood || !e.HasMemory {
		t.Fatal("chat")
	}
	if e := Effects(DeepTalk); e.Memory != MemDeepTalk {
		t.Fatal("deep")
	}
	if e := Effects(Proposal); e.HasMemory || e.HasMood {
		t.Fatal("proposal")
	}
	for _, tc := range []struct {
		skill   int
		talking int32
		want    int32
	}{{0, 1000, 820}, {10, 1000, 1095}, {20, 1000, 1370}, {20, 500, 685}, {0, 100, 82}} {
		if got := SocialImpact(tc.skill, tc.talking); got != tc.want {
			t.Errorf("impact(%d,%d)=%d want %d", tc.skill, tc.talking, got, tc.want)
		}
	}
	if SocialImpact(-20, 1000) != 270 && SocialImpact(-40, 1000) != 200 {
		t.Fatal("floor")
	}
	if Chitchat.String() != "chitchat" || NumInteractions.String() != "" {
		t.Fatal("String")
	}
}

func TestPairTransitions(t *testing.T) {
	valid := map[[2]PairKind]bool{
		{PairNone, PairLover}: true, {PairLover, PairFiance}: true, {PairLover, PairExLover}: true,
		{PairFiance, PairSpouse}: true, {PairFiance, PairExLover}: true, {PairSpouse, PairExSpouse}: true,
		{PairExLover, PairLover}: true, {PairExSpouse, PairLover}: true,
	}
	for from := PairNone; from <= PairExSpouse; from++ {
		for to := PairNone; to <= PairExSpouse; to++ {
			if CanTransition(from, to) != valid[[2]PairKind{from, to}] {
				t.Errorf("%d->%d", from, to)
			}
		}
	}
	pr := Pair{}
	if pr.Transition(PairSpouse, 5) {
		t.Fatal("none->spouse")
	}
	if !pr.Transition(PairLover, 10) || pr.Since != 10 || pr.LastChange != 10 {
		t.Fatal("lover")
	}
	if pr.CanBreakUp(10 + BreakupGrace - 1) {
		t.Fatal("grace")
	}
	if !pr.CanBreakUp(10 + BreakupGrace) {
		t.Fatal("after grace")
	}
	if !pr.CanPropose(0) {
		t.Fatal("propose")
	}
	pr.RejectProposal(100)
	if pr.CanPropose(100+ProposalCooldown-1) || !pr.CanPropose(100+ProposalCooldown) {
		t.Fatal("cooldown")
	}
	pr.Rebuff(50)
	if pr.CanRomance(50+RebuffCooldown-1) || !pr.CanRomance(50+RebuffCooldown) {
		t.Fatal("rebuff")
	}
	pr.Transition(PairExLover, 500)
	if pr.Since != 10 || pr.Kind != PairExLover || pr.CanBreakUp(1<<40) || pr.CanPropose(1<<40) {
		t.Fatal("ex")
	}
	if !pr.Transition(PairLover, 600) || pr.Since != 600 {
		t.Fatal("reconcile")
	}
	for k, want := range map[PairKind]Kind{PairLover: Lover, PairFiance: Fiance, PairSpouse: Spouse, PairExLover: ExLover, PairExSpouse: ExSpouse} {
		if got, ok := k.Relation(); !ok || got != want {
			t.Errorf("Relation(%d)", k)
		}
	}
	if _, ok := PairNone.Relation(); ok {
		t.Fatal("none relation")
	}
	if PairExLover.Active() || !PairFiance.Active() {
		t.Fatal("Active")
	}
}

func TestAttraction(t *testing.T) {
	m, f := p(1, true), p(2, false)
	tests := []struct {
		name string
		a, b Person
		kin  bool
		op   int32
		pos  bool
	}{
		{"straight opposite", m, f, false, 3000, true},
		{"straight same", m, p(3, true), false, 3000, false},
		{"gay same", p(1, true, trait.Gay), p(3, true), false, 3000, true},
		{"gay opposite", p(1, true, trait.Gay), f, false, 3000, false},
		{"bi same", p(1, true, trait.Bisexual), p(3, true), false, 3000, true},
		{"bi opposite", p(1, true, trait.Bisexual), f, false, 3000, true},
		{"asexual", p(1, true, trait.Asexual), f, false, 3000, false},
		{"kin", m, f, true, 3000, false},
		{"hates", m, f, false, -3000, false},
		{"self", m, m, false, 3000, false},
	}
	for _, c := range tests {
		if got := Attraction(c.a, c.b, c.kin, c.op) > 0; got != c.pos {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	young := f
	young.AgeYears = 15
	old := f
	old.AgeYears = 80
	if Attraction(m, young, false, 3000) != 0 || Attraction(young, m, false, 3000) != 0 {
		t.Fatal("minor")
	}
	if Attraction(m, old, false, 3000) != 0 {
		t.Fatal("age gap 50")
	}
	near := f
	near.AgeYears = 33
	mid := f
	mid.AgeYears = 40
	if Attraction(m, near, false, 3000) <= Attraction(m, mid, false, 3000) {
		t.Fatal("age curve")
	}
	pretty, ugly, dis := f, f, f
	pretty.Beauty, ugly.Beauty, dis.Disfigured = 2, -2, true
	if !(Attraction(m, pretty, false, 3000) > Attraction(m, f, false, 3000) &&
		Attraction(m, f, false, 3000) > Attraction(m, ugly, false, 3000) &&
		Attraction(m, f, false, 3000) > Attraction(m, dis, false, 3000)) {
		t.Fatal("beauty ordering")
	}
	blind := m
	blind.Blind = true
	if Attraction(blind, pretty, false, 3000) != Attraction(blind, f, false, 3000) && Attraction(blind, pretty, false, 3000) != 1000 {
		t.Fatal("blind ignores beauty")
	}
	extreme := f
	extreme.Beauty = -128
	extreme.Disfigured = true
	if Attraction(m, extreme, false, 3000) <= 0 {
		t.Fatal("floor")
	}
	dead := f
	dead.Dead = true
	if Attraction(m, dead, false, 3000) != 0 {
		t.Fatal("dead")
	}
	if Attraction(m, f, false, 5000) <= Attraction(m, f, false, 1000) {
		t.Fatal("opinion")
	}
	if Attraction(m, f, false, 1000) == Attraction(f, m, false, 6000) {
		t.Log("symmetric by coincidence")
	}
}

func TestRomanceChances(t *testing.T) {
	if RomanceSuccess(0, false) != 0 || RomanceSuccess(500, false) != 500 || RomanceSuccess(5000, false) != 950 || RomanceSuccess(800, true) != 200 {
		t.Fatal("success")
	}
	if BreakupWeight(-10000, false) != 4000 || BreakupWeight(-10000, true) != 4300 || BreakupWeight(5000, false) != 0 {
		t.Fatal("breakup")
	}
}

func TestGatheringWedding(t *testing.T) {
	g := Gathering{Kind: Wedding, Hosts: [2]ID{1, 2}}
	now := uint64(1000)
	if g.Advance(now, false, false) || g.Phase != PhaseScheduled {
		t.Fatal("waits")
	}
	if !g.Advance(now, true, false) || g.Phase != PhaseGathering || g.BothArrivedAt != now {
		t.Fatal("arrive")
	}
	if g.Advance(now+WeddingGatherMin-1, true, false) {
		t.Fatal("too early")
	}
	// A host leaves: back to calling, arrival resets.
	if !g.Advance(now+10, false, false) || g.Phase != PhaseCalling || g.BothArrivedAt != 0 {
		t.Fatal("host left")
	}
	now += 100
	g.Advance(now, true, false)
	if !g.Advance(now+WeddingGatherMin, true, false) || g.Phase != PhaseCeremony {
		t.Fatal("ceremony")
	}
	at := g.PhaseAt
	if g.Advance(at+WeddingCeremony-1, true, false) || !g.Advance(at+WeddingCeremony, true, false) || g.Phase != PhaseReception {
		t.Fatal("reception")
	}
	at = g.PhaseAt
	if g.Advance(at+WeddingReception-1, true, false) || !g.Advance(at+WeddingReception, true, false) || g.Phase != PhaseDone {
		t.Fatal("done")
	}
	if !g.Finished() || g.Advance(at+1<<30, true, false) || g.Advance(at+1<<30, true, true) {
		t.Fatal("terminal")
	}
}

func TestGatheringPartyAndAbort(t *testing.T) {
	g := Gathering{Kind: Party, Hosts: [2]ID{1, 0}}
	g.Advance(0, true, false)
	if g.Phase != PhaseGathering {
		t.Fatal("party starts")
	}
	if g.Advance(PartyDuration-1, false, false) || !g.Advance(PartyDuration, false, false) || g.Phase != PhaseDone {
		t.Fatal("party ends")
	}
	for _, ph := range []GatherPhase{PhaseCalling, PhaseUnderway, PhaseCeremony, PhaseReception} {
		w := Gathering{Kind: Wedding, Phase: ph}
		if !w.Advance(5000, true, true) || w.Phase != PhaseAborted || !w.Finished() {
			t.Fatalf("abort from %d", ph)
		}
		if w.RetryAt() != 5000+Day {
			t.Fatal("retry")
		}
	}
}

func TestFight(t *testing.T) {
	if FightChance(Insult) != 40 || FightChance(Slight) != 5 || FightChance(Chitchat) != 0 || FightChance(DeepTalk, 4000) != 0 {
		t.Fatal("base")
	}
	if FightChance(Insult, BloodthirstyMult) != 160 || FightChance(Slight, BloodthirstyMult) != 20 {
		t.Fatal("bloodthirsty")
	}
	if FightChance(Insult, 1000, MalnutritionMult(4)) != 120 || FightChance(Insult, BloodthirstyMult, 3000, 3000) != 1000 {
		t.Fatal("stack/cap")
	}
	if FightChance(Insult, -5000) != 0 {
		t.Fatal("negative")
	}
	for band, want := range map[int]int32{-1: 1000, 0: 1000, 1: 1500, 2: 2000, 3: 2500, 4: 3000, 9: 3000} {
		if MalnutritionMult(band) != want {
			t.Errorf("band %d", band)
		}
	}
	if RollOutcome(fixedRoll(0)) != FightAngering || RollOutcome(fixedRoll(999)) != FightCathartic {
		t.Fatal("outcome")
	}
	if m, d := FightAngering.Mood(); m != -22 || d != 20 {
		t.Fatal("angering")
	}
	if m, d := FightCathartic.Mood(); m != 38 || d != 20 {
		t.Fatal("cathartic")
	}
	if FightAngering.Memory() != MemAngeringFight || FightCathartic.Memory() != MemCatharticFight {
		t.Fatal("memory")
	}
}

func TestGrief(t *testing.T) {
	for k, want := range map[Kind]int{Spouse: -20, Fiance: -18, Lover: -16, Child: -20, Sibling: -14, Grandchild: -12,
		Parent: -8, HalfSibling: -5, NieceNephew: -5, AuntUncle: -4, Grandparent: -4, Cousin: -4, Kin: -4} {
		d, ok := GriefFor(k, false, false, false)
		if !ok || d.Mood != want || d.Days != 30 {
			t.Errorf("%d: %+v", k, d)
		}
	}
	if d, ok := GriefFor(NoKind, true, false, false); !ok || d.Mood != -10 || d.Days != 20 {
		t.Fatal("friend")
	}
	if d, ok := GriefFor(NoKind, false, true, false); !ok || d.Mood != 10 || d.Days != 10 {
		t.Fatal("rival")
	}
	if _, ok := GriefFor(NoKind, false, false, false); ok {
		t.Fatal("stranger")
	}
	if _, ok := GriefFor(ExLover, false, false, false); ok {
		t.Fatal("ex")
	}
	if d, _ := GriefFor(Parent, true, false, false); d.Mood != -8 {
		t.Fatal("relation wins")
	}
	if d, ok := GriefFor(Spouse, false, false, true); !ok || d.Mood != -20 || d.Days != 30 {
		t.Fatalf("lost %+v", d)
	}
	if d, ok := GriefFor(NoKind, false, true, true); !ok || d.Mood != 10 || d.Days != 10 {
		t.Fatalf("rival lost %+v", d)
	}
	if _, ok := GriefFor(NoKind, false, false, true); ok {
		t.Fatal("stranger lost")
	}
	if d, ok := GriefFor(Kin, false, false, true); !ok || d.Mood != -4 || d.Days != 30 {
		t.Fatal("kin lost")
	}
	if d, ok := GriefFor(NoKind, true, false, true); !ok || d.Days != 20 || d.Mood != -10 {
		t.Fatalf("friend lost %+v", d)
	}
	if !ExemptFromGrief([]trait.ID{trait.Sunny, trait.ColdHearted}) || ExemptFromGrief(nil) {
		t.Fatal("exempt")
	}
	for k, want := range map[Kind]int32{Parent: -8000, Child: -8000, Sibling: -8000, Spouse: -6500, Fiance: -6500,
		Lover: -5000, Cousin: -1500, HalfSibling: -1500, ExLover: 0, ExSpouse: 0, NumKinds: 0} {
		if KillerOpinion(k) != want {
			t.Errorf("killer %d", k)
		}
	}
	for k, want := range map[Kind]MemKind{Parent: MemKilledMyParentChild, Child: MemKilledMyParentChild, Sibling: MemKilledMySibling,
		Spouse: MemKilledMySpouse, Lover: MemKilledMyLover, Cousin: MemKilledMyKin} {
		if got, ok := KillerMemory(k); !ok || got != want {
			t.Errorf("killer mem %d", k)
		}
	}
	if _, ok := KillerMemory(ExLover); ok {
		t.Fatal("ex mem")
	}
	if _, ok := KillerMemory(NumKinds); ok {
		t.Fatal("none mem")
	}
}

func colonists() []Person {
	var out []Person
	for i := 0; i < 8; i++ {
		out = append(out, Person{ID: ID(i + 1), Male: i%2 == 0, AgeYears: 20 + i*6})
	}
	return out
}

func TestFamiliesDeterministicAndConsistent(t *testing.T) {
	cs := colonists()
	a := Families(42, cs, nil, 1000)
	b := Families(42, cs, nil, 1000)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("not deterministic")
	}
	// Across many seeds: sexes/ages consistent, no self links.
	anyParents, anyMarriage, anyGP := false, false, false
	byID := map[ID]Person{}
	for _, c := range cs {
		byID[c.ID] = c
	}
	for s := uint64(0); s < 300; s++ {
		pl := Families(s, cs, nil, 1000)
		specs := map[ID]PersonSpec{}
		for _, sp := range pl.Specs {
			if _, dup := specs[sp.ID]; dup || sp.ID < 1000 {
				t.Fatalf("spec id %d", sp.ID)
			}
			specs[sp.ID] = sp
		}
		age := func(id ID) int {
			if c, ok := byID[id]; ok {
				return c.AgeYears
			}
			return specs[id].AgeYears
		}
		male := func(id ID) bool {
			if c, ok := byID[id]; ok {
				return c.Male
			}
			return specs[id].Male
		}
		for _, l := range pl.Parents {
			anyParents = true
			if _, ok := specs[l.Child]; ok && specs[l.Child].AgeYears == 0 {
				t.Fatal("spec age")
			}
			if l.Parents[0] != 0 && male(l.Parents[0]) {
				t.Fatalf("seed %d: mother %d male", s, l.Parents[0])
			}
			if l.Parents[1] != 0 && !male(l.Parents[1]) {
				t.Fatalf("seed %d: father %d female", s, l.Parents[1])
			}
			for _, p := range l.Parents {
				if p != 0 && age(p)-age(l.Child) < 15 {
					t.Fatalf("seed %d: parent %d (%d) child %d (%d)", s, p, age(p), l.Child, age(l.Child))
				}
				if p == l.Child {
					t.Fatal("self parent")
				}
			}
			if _, ok := specs[l.Child]; ok {
				for _, o := range pl.Parents {
					if o.Parents[0] == l.Child || o.Parents[1] == l.Child {
						anyGP = true
					}
				}
			}
		}
		married := map[ID]bool{}
		for _, m := range pl.Marriages {
			anyMarriage = true
			if m.A == m.B || married[m.A] || married[m.B] {
				t.Fatal("bad marriage")
			}
			married[m.A], married[m.B] = true, true
		}
		for _, h := range pl.Surnames {
			if h.Person == h.From {
				t.Fatal("self surname")
			}
		}
	}
	if !anyParents || !anyMarriage || !anyGP {
		t.Fatalf("coverage %v %v %v", anyParents, anyMarriage, anyGP)
	}
	if len(Families(1, nil, nil, 5).Specs) != 0 {
		t.Fatal("empty")
	}
}

func TestFamiliesColonistTies(t *testing.T) {
	cs := colonists()
	sib, par, wed := false, false, false
	for s := uint64(0); s < 400; s++ {
		pl := Families(s, cs, nil, 1000)
		kids := map[[2]ID][]ID{}
		for _, l := range pl.Parents {
			if l.Child <= 8 {
				kids[l.Parents] = append(kids[l.Parents], l.Child)
				if l.Parents[0] != 0 && l.Parents[0] <= 8 || l.Parents[1] != 0 && l.Parents[1] <= 8 {
					par = true
				}
			}
		}
		for _, v := range kids {
			if len(v) > 1 {
				sib = true
			}
		}
		if len(pl.Marriages) > 0 {
			wed = true
		}
	}
	if !sib || !par || !wed {
		t.Fatalf("sib=%v par=%v wed=%v", sib, par, wed)
	}
	// Provided roll is used.
	n := 0
	roll := func(int) int { n++; return 999 }
	Families(1, cs, roll, 1)
	if n == 0 {
		t.Fatal("roll unused")
	}
	// Always-hit roll exercises branches.
	zero := func(int) int { return 0 }
	pl := Families(1, cs, zero, 1)
	if len(pl.Parents) == 0 {
		t.Fatal("zero roll")
	}
}

func TestSpecValues(t *testing.T) {
	for k, want := range map[Kind]int32{Parent: 3000, Child: 3000, Sibling: 2000, HalfSibling: 1500, Grandparent: 1500, Grandchild: 1500,
		AuntUncle: 1000, NieceNephew: 1000, Cousin: 1000, GreatGrandparent: 500, GreatGrandchild: 500, GrandAuntUncle: 500,
		GrandNieceNephew: 500, SecondCousin: 500, Kin: 500, StepParent: 500, StepChild: 500, ParentInLaw: 500, ChildInLaw: 500,
		Lover: 3500, Fiance: 3000, Spouse: 3000, ExLover: -1500, ExSpouse: -1500} {
		if Bonus(k) != want {
			t.Errorf("bonus %d=%d want %d", k, Bonus(k), want)
		}
	}
	for k, want := range map[MemKind][2]int64{MemDeepTalk: {1500, 20}, MemSlighted: {-500, 20}, MemInsulted: {-1500, 20},
		MemKindWords: {1500, 20}, MemRebuffedMe: {-1000, 3}, MemFailedRomance: {-1500, 3}, MemCheatedOnMe: {-7000, 25},
		MemRejectedMyProposal: {-3000, 25}, MemIRejectedProposal: {-1500, 25}, MemBrokeUpWithMe: {-5000, 25},
		MemDivorcedMe: {-7000, 30}, MemAngeringFight: {-2200, 20}, MemCatharticFight: {3800, 20}, MemHarmedMe: {-1500, 10},
		MemRescuedMe: {1500, 30}, MemRescuedMeStranger: {2500, 30}, MemRecruitedMe: {2000, 120}, MemBotchedSurgery: {-2000, 90}, MemChitchat: {66, 6}} {
		d := MemDef(k)
		if int64(d.Value) != want[0] || int64(d.Duration) != want[1]*Day {
			t.Errorf("mem %d: %+v want %v", k, d, want)
		}
	}
}

// §13.2 Social impact: a memory lands at its Scale (0 = full, as in older
// saves), stacking still diminishing per copy; ImpactScale clamps.
func TestMemoryScale(t *testing.T) {
	full := AddMemory(nil, MemDeepTalk, 2, 0)
	if full[0].Scale != 0 || Sum(full, 2, 0) != memDeepTalk {
		t.Fatalf("unscaled deep talk %d (scale %d)", Sum(full, 2, 0), full[0].Scale)
	}
	big := AddScaledMemory(nil, MemDeepTalk, 2, 0, ImpactScale(1370))
	small := AddScaledMemory(nil, MemDeepTalk, 2, 0, ImpactScale(820))
	if Sum(big, 2, 0) != memDeepTalk*1370/1000 || Sum(small, 2, 0) != memDeepTalk*820/1000 {
		t.Fatalf("scaled deep talk %d / %d", Sum(big, 2, 0), Sum(small, 2, 0))
	}
	ins := AddScaledMemory(nil, MemInsulted, 2, 0, 1500)
	ins = AddScaledMemory(ins, MemInsulted, 2, 1, 1500)
	if want := int32(memInsulted*1500/1000 + memInsulted*1500/1000*stackFactorSocial/1000); Sum(ins, 2, 0) != want {
		t.Fatalf("two harsh insults %d, want %d (the negative magnitude scales too)", Sum(ins, 2, 0), want)
	}
	for in, want := range map[int32]int16{0: MinMemoryScale, 100: MinMemoryScale, 1000: 0, 1200: 1200, 5000: MaxMemoryScale} {
		if got := ImpactScale(in); got != want {
			t.Errorf("ImpactScale(%d) = %d, want %d", in, got, want)
		}
	}
	// a refreshed copy at the stack limit takes the new scale
	var rs []OpinionMemory
	rs = AddScaledMemory(rs, MemRebuffedMe, 1, 0, 500)
	rs = AddScaledMemory(rs, MemRebuffedMe, 1, 10, 1500)
	if len(rs) != 1 || rs[0].Scale != 1500 {
		t.Fatalf("refresh %+v", rs)
	}
}

// Breakup pressure grows with a known affair; proposals with time
// together, and so does the chance one is accepted.
func TestRomanceWeightsAffairAndTogether(t *testing.T) {
	c := InteractCtx{Initiator: p(1, true), Pair: PairLover, OpinionIR: 0}
	plain := Weights(c)[Breakup]
	c.Affair = true
	if cheated := Weights(c)[Breakup]; cheated <= plain || cheated != int64(WeightBreakup)*int64(BreakupWeight(0, true))/1000 {
		t.Fatalf("breakup weight %d with affair vs %d", cheated, plain)
	}
	c = InteractCtx{Initiator: p(1, true), Pair: PairLover, OpinionIR: 6000}
	fresh := Weights(c)[Proposal]
	c.Together = 30 * Day
	long := Weights(c)[Proposal]
	if fresh <= 0 || long <= fresh*5 {
		t.Fatalf("proposal weight fresh %d, 30 days %d", fresh, long)
	}
	c.Pair = PairSpouse
	if Weights(c)[Proposal] != 0 {
		t.Fatal("a married couple proposes")
	}
	if a, b := ProposalAcceptFor(4000, 0), ProposalAcceptFor(4000, 40*Day); a >= b || b > 1000 {
		t.Fatalf("accept fresh %d, long %d", a, b)
	}
	if ProposalAcceptFor(20000, 100*Day) != 1000 || ProposalAcceptFor(-100, 100*Day) != 0 {
		t.Fatal("accept bounds")
	}
}

// Grief lines name the relation: "my wife died", "my son is missing",
// "my friend died", "my rival died"; none without a grief.
func TestGriefLabel(t *testing.T) {
	cases := []struct {
		k                         Kind
		male, friend, rival, lost bool
		want                      string
	}{
		{Spouse, false, false, false, false, "my wife died"},
		{Child, true, false, false, true, "my son is missing"},
		{Sibling, true, true, false, false, "my brother died"},
		{NoKind, false, true, false, false, "my friend died"},
		{NoKind, true, false, true, false, "my rival died"},
		{StepParent, true, true, false, false, "my friend died"},
		{NoKind, true, false, false, false, ""},
		{ExLover, true, false, false, false, ""},
	}
	for _, c := range cases {
		if got := GriefLabel(c.k, c.male, c.friend, c.rival, c.lost); got != c.want {
			t.Errorf("GriefLabel(%v) = %q, want %q", c, got, c.want)
		}
	}
}
