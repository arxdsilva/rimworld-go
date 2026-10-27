package body

// Body plans (MED-13). A Body reads its parts from the table of its
// Anatomy: the humanlike table (anatomy.go, unchanged) or one of the
// animal plans below. Part values index the body's own table, so the
// same number names different parts in different plans; each plan has at
// most 32 parts (Missing and Fresh are uint32 bit sets).
//
// Hit points and coverage are original tuning. MaxHP is given at a
// human's scale (100 hit points); a body's Scale (permille, 0 meaning
// 1000) scales every part, so a hare's leg holds less than a bison's.

// Anatomy is a body plan (saved: append only). The zero value is the
// humanlike body, so human bodies and their saves are unchanged.
type Anatomy uint8

const (
	Humanlike Anatomy = iota
	Quadruped
	Bird
	Snake
	Lizard
	Insect
	NumAnatomies
)

var anatomyNames = [NumAnatomies]string{"humanlike", "quadruped", "bird", "snake", "lizard", "insect"}

func (a Anatomy) String() string {
	if a < NumAnatomies {
		return anatomyNames[a]
	}
	return "humanlike"
}

// Quadruped parts.
const (
	QHead Part = iota
	QBrain
	QEyeL
	QEyeR
	QEarL
	QEarR
	QNose
	QJaw
	QTorso
	QHeart
	QLungL
	QLungR
	QStomach
	QLiver
	QKidneyL
	QKidneyR
	QSpine
	QLegFL
	QLegFR
	QLegHL
	QLegHR
	QPawFL
	QPawFR
	QPawHL
	QPawHR
	QTail
	numQuadruped
)

// Bird parts: wings where a quadruped has its front legs, and a beak.
const (
	BHead Part = iota
	BBrain
	BEyeL
	BEyeR
	BEarL
	BEarR
	BBeak
	BTorso
	BHeart
	BLungL
	BLungR
	BStomach
	BLiver
	BKidneyL
	BKidneyR
	BSpine
	BWingL
	BWingR
	BLegL
	BLegR
	BFootL
	BFootR
	BTail
	numBird
)

// Snake parts: no limbs; the spine moves the body.
const (
	SHead Part = iota
	SBrain
	SEyeL
	SEyeR
	STongue
	SJaw
	SBody
	SHeart
	SLung
	SStomach
	SLiver
	SKidneyL
	SKidneyR
	SSpine
	STail
	numSnake
)

// Lizard parts (lizards and tortoises): four legs and feet, a long tail.
const (
	LHead Part = iota
	LBrain
	LEyeL
	LEyeR
	LJaw
	LTorso
	LHeart
	LLungL
	LLungR
	LStomach
	LLiver
	LKidneyL
	LKidneyR
	LSpine
	LLegFL
	LLegFR
	LLegHL
	LLegHR
	LFootFL
	LFootFR
	LFootHL
	LFootHR
	LTail
	numLizard
)

// Insect parts: six legs, antennae and a carapace over the thorax.
const (
	IHead Part = iota
	IBrain
	IEyeL
	IEyeR
	IAntennaL
	IAntennaR
	IMandibles
	IThorax
	ICarapace
	IHeart
	IStomach
	INerveCord
	ILegFL
	ILegFR
	ILegML
	ILegMR
	ILegHL
	ILegHR
	numInsect
)

// The plans' part tables. Each sums to 1000 coverage; the root (torso,
// body, thorax) takes what the other parts leave.
var (
	quadrupedDefs = [numQuadruped]Def{
		QHead:    {"head", QTorso, 30, 60, true, true, 0, false},
		QBrain:   {"brain", QHead, 15, 25, true, false, Consciousness, true},
		QEyeL:    {"left eye", QHead, 10, 10, false, true, Sight, true},
		QEyeR:    {"right eye", QHead, 10, 10, false, true, Sight, true},
		QEarL:    {"left ear", QHead, 10, 10, false, true, Hearing, true},
		QEarR:    {"right ear", QHead, 10, 10, false, true, Hearing, true},
		QNose:    {"nose", QHead, 10, 10, false, true, 0, false},
		QJaw:     {"jaw", QHead, 20, 20, false, true, 0, false},
		QTorso:   {"torso", QTorso, 50, 298, true, false, 0, false},
		QHeart:   {"heart", QTorso, 25, 18, true, false, BloodPumping, true},
		QLungL:   {"left lung", QTorso, 30, 25, false, false, Breathing, true},
		QLungR:   {"right lung", QTorso, 30, 25, false, false, Breathing, true},
		QStomach: {"stomach", QTorso, 30, 25, false, false, Digestion, true},
		QLiver:   {"liver", QTorso, 40, 22, false, false, BloodFiltration, true},
		QKidneyL: {"left kidney", QTorso, 35, 15, false, false, BloodFiltration, true},
		QKidneyR: {"right kidney", QTorso, 35, 15, false, false, BloodFiltration, true},
		QSpine:   {"spine", QTorso, 40, 22, false, false, Moving, true},
		QLegFL:   {"front left leg", QTorso, 30, 70, false, false, Moving, true},
		QLegFR:   {"front right leg", QTorso, 30, 70, false, false, Moving, true},
		QLegHL:   {"hind left leg", QTorso, 30, 70, false, false, Moving, true},
		QLegHR:   {"hind right leg", QTorso, 30, 70, false, false, Moving, true},
		QPawFL:   {"front left paw", QLegFL, 15, 20, false, false, Moving, true},
		QPawFR:   {"front right paw", QLegFR, 15, 20, false, false, Moving, true},
		QPawHL:   {"hind left paw", QLegHL, 15, 20, false, false, Moving, true},
		QPawHR:   {"hind right paw", QLegHR, 15, 20, false, false, Moving, true},
		QTail:    {"tail", QTorso, 10, 20, false, false, 0, false},
	}
	birdDefs = [numBird]Def{
		BHead:    {"head", BTorso, 25, 50, true, true, 0, false},
		BBrain:   {"brain", BHead, 12, 25, true, false, Consciousness, true},
		BEyeL:    {"left eye", BHead, 8, 10, false, true, Sight, true},
		BEyeR:    {"right eye", BHead, 8, 10, false, true, Sight, true},
		BEarL:    {"left ear", BHead, 8, 5, false, true, Hearing, true},
		BEarR:    {"right ear", BHead, 8, 5, false, true, Hearing, true},
		BBeak:    {"beak", BHead, 15, 25, false, true, 0, false},
		BTorso:   {"torso", BTorso, 45, 308, true, false, 0, false},
		BHeart:   {"heart", BTorso, 22, 18, true, false, BloodPumping, true},
		BLungL:   {"left lung", BTorso, 25, 25, false, false, Breathing, true},
		BLungR:   {"right lung", BTorso, 25, 25, false, false, Breathing, true},
		BStomach: {"stomach", BTorso, 25, 25, false, false, Digestion, true},
		BLiver:   {"liver", BTorso, 35, 22, false, false, BloodFiltration, true},
		BKidneyL: {"left kidney", BTorso, 30, 15, false, false, BloodFiltration, true},
		BKidneyR: {"right kidney", BTorso, 30, 15, false, false, BloodFiltration, true},
		BSpine:   {"spine", BTorso, 35, 22, false, false, Moving, true},
		BWingL:   {"left wing", BTorso, 25, 90, false, false, 0, false},
		BWingR:   {"right wing", BTorso, 25, 90, false, false, 0, false},
		BLegL:    {"left leg", BTorso, 25, 75, false, false, Moving, true},
		BLegR:    {"right leg", BTorso, 25, 75, false, false, Moving, true},
		BFootL:   {"left foot", BLegL, 12, 25, false, false, Moving, true},
		BFootR:   {"right foot", BLegR, 12, 25, false, false, Moving, true},
		BTail:    {"tail", BTorso, 8, 15, false, false, 0, false},
	}
	// snakeDefs: the spine runs the whole body and losing it kills (it
	// is vital, so a hit never takes it off: the damage spills on).
	snakeDefs = [numSnake]Def{
		SHead:    {"head", SBody, 25, 70, true, true, 0, false},
		SBrain:   {"brain", SHead, 12, 25, true, false, Consciousness, true},
		SEyeL:    {"left eye", SHead, 8, 10, false, true, Sight, true},
		SEyeR:    {"right eye", SHead, 8, 10, false, true, Sight, true},
		STongue:  {"tongue", SHead, 8, 10, false, true, 0, false},
		SJaw:     {"jaw", SHead, 15, 25, false, true, 0, false},
		SBody:    {"body", SBody, 60, 510, true, false, 0, false},
		SHeart:   {"heart", SBody, 22, 20, true, false, BloodPumping, true},
		SLung:    {"lung", SBody, 35, 45, false, false, Breathing, true},
		SStomach: {"stomach", SBody, 30, 35, false, false, Digestion, true},
		SLiver:   {"liver", SBody, 35, 30, false, false, BloodFiltration, true},
		SKidneyL: {"left kidney", SBody, 30, 15, false, false, BloodFiltration, true},
		SKidneyR: {"right kidney", SBody, 30, 15, false, false, BloodFiltration, true},
		SSpine:   {"spine", SBody, 50, 60, true, false, Moving, true},
		STail:    {"tail", SBody, 25, 120, false, false, 0, false},
	}
	lizardDefs = [numLizard]Def{
		LHead:    {"head", LTorso, 25, 60, true, true, 0, false},
		LBrain:   {"brain", LHead, 12, 25, true, false, Consciousness, true},
		LEyeL:    {"left eye", LHead, 8, 10, false, true, Sight, true},
		LEyeR:    {"right eye", LHead, 8, 10, false, true, Sight, true},
		LJaw:     {"jaw", LHead, 15, 20, false, true, 0, false},
		LTorso:   {"torso", LTorso, 50, 316, true, false, 0, false},
		LHeart:   {"heart", LTorso, 22, 18, true, false, BloodPumping, true},
		LLungL:   {"left lung", LTorso, 25, 25, false, false, Breathing, true},
		LLungR:   {"right lung", LTorso, 25, 25, false, false, Breathing, true},
		LStomach: {"stomach", LTorso, 25, 25, false, false, Digestion, true},
		LLiver:   {"liver", LTorso, 35, 22, false, false, BloodFiltration, true},
		LKidneyL: {"left kidney", LTorso, 30, 15, false, false, BloodFiltration, true},
		LKidneyR: {"right kidney", LTorso, 30, 15, false, false, BloodFiltration, true},
		LSpine:   {"spine", LTorso, 35, 22, false, false, Moving, true},
		LLegFL:   {"front left leg", LTorso, 25, 60, false, false, Moving, true},
		LLegFR:   {"front right leg", LTorso, 25, 60, false, false, Moving, true},
		LLegHL:   {"hind left leg", LTorso, 25, 60, false, false, Moving, true},
		LLegHR:   {"hind right leg", LTorso, 25, 60, false, false, Moving, true},
		LFootFL:  {"front left foot", LLegFL, 12, 18, false, false, Moving, true},
		LFootFR:  {"front right foot", LLegFR, 12, 18, false, false, Moving, true},
		LFootHL:  {"hind left foot", LLegHL, 12, 18, false, false, Moving, true},
		LFootHR:  {"hind right foot", LLegHR, 12, 18, false, false, Moving, true},
		LTail:    {"tail", LTorso, 20, 80, false, false, 0, false},
	}
	// insectDefs: the antennae stand in for hearing; an insect breathes
	// through its shell and has no filtering organs (both stay full).
	insectDefs = [numInsect]Def{
		IHead:      {"head", IThorax, 25, 60, true, true, 0, false},
		IBrain:     {"brain", IHead, 10, 20, true, false, Consciousness, true},
		IEyeL:      {"left eye", IHead, 8, 10, false, true, Sight, true},
		IEyeR:      {"right eye", IHead, 8, 10, false, true, Sight, true},
		IAntennaL:  {"left antenna", IHead, 8, 10, false, true, Hearing, true},
		IAntennaR:  {"right antenna", IHead, 8, 10, false, true, Hearing, true},
		IMandibles: {"mandibles", IHead, 15, 25, false, true, 0, false},
		IThorax:    {"thorax", IThorax, 50, 270, true, false, 0, false},
		ICarapace:  {"carapace", IThorax, 40, 180, false, false, 0, false},
		IHeart:     {"heart", IThorax, 22, 20, true, false, BloodPumping, true},
		IStomach:   {"stomach", IThorax, 25, 30, false, false, Digestion, true},
		INerveCord: {"nerve cord", IThorax, 30, 25, false, false, Moving, true},
		ILegFL:     {"front left leg", IThorax, 20, 55, false, false, Moving, true},
		ILegFR:     {"front right leg", IThorax, 20, 55, false, false, Moving, true},
		ILegML:     {"middle left leg", IThorax, 20, 55, false, false, Moving, true},
		ILegMR:     {"middle right leg", IThorax, 20, 55, false, false, Moving, true},
		ILegHL:     {"hind left leg", IThorax, 20, 55, false, false, Moving, true},
		ILegHR:     {"hind right leg", IThorax, 20, 55, false, false, Moving, true},
	}
)

// capRule is how a plan's capacity reads its parts: the average of each
// chain's product (a leg times its paw), times every part in mul (the
// spine). A rule with neither leaves the capacity full: the plan has no
// part serving it.
type capRule struct {
	chains [][]Part
	mul    []Part
}

func (r capRule) empty() bool { return len(r.chains) == 0 && len(r.mul) == 0 }

// parts is every part the rule reads, chains first.
func (r capRule) parts() []Part {
	var out []Part
	for _, ch := range r.chains {
		out = append(out, ch...)
	}
	return append(out, r.mul...)
}

// plan is an animal anatomy: its table, its root (where whole-body
// wounds land), brain and heart, and its capacity rules. Consciousness
// reads the brain and blood pumping the heart, as the humanlike body.
type plan struct {
	defs  []Def
	root  Part
	brain Part
	heart Part
	caps  [NumCaps]capRule
}

func one(ps ...Part) [][]Part {
	out := make([][]Part, len(ps))
	for i, p := range ps {
		out[i] = []Part{p}
	}
	return out
}

// plans are the animal anatomies (the humanlike body keeps its own code
// path in capacity.go; its entry here only names the root).
var plans = [NumAnatomies]plan{
	Humanlike: {defs: defs[:], root: Torso, brain: Brain, heart: Heart},
	Quadruped: {defs: quadrupedDefs[:], root: QTorso, brain: QBrain, heart: QHeart, caps: [NumCaps]capRule{
		Moving:          {chains: [][]Part{{QLegFL, QPawFL}, {QLegFR, QPawFR}, {QLegHL, QPawHL}, {QLegHR, QPawHR}}, mul: []Part{QSpine}},
		Sight:           {chains: one(QEyeL, QEyeR)},
		Hearing:         {chains: one(QEarL, QEarR)},
		Breathing:       {chains: one(QLungL, QLungR)},
		BloodPumping:    {chains: one(QHeart)},
		BloodFiltration: {chains: one(QKidneyL, QKidneyR), mul: []Part{QLiver}},
		Digestion:       {chains: one(QStomach), mul: []Part{QLiver}},
	}},
	// Bird: the wings serve no capacity (these birds do not fly); it
	// walks on its two legs.
	Bird: {defs: birdDefs[:], root: BTorso, brain: BBrain, heart: BHeart, caps: [NumCaps]capRule{
		Moving:          {chains: [][]Part{{BLegL, BFootL}, {BLegR, BFootR}}, mul: []Part{BSpine}},
		Sight:           {chains: one(BEyeL, BEyeR)},
		Hearing:         {chains: one(BEarL, BEarR)},
		Breathing:       {chains: one(BLungL, BLungR)},
		BloodPumping:    {chains: one(BHeart)},
		BloodFiltration: {chains: one(BKidneyL, BKidneyR), mul: []Part{BLiver}},
		Digestion:       {chains: one(BStomach), mul: []Part{BLiver}},
	}},
	Snake: {defs: snakeDefs[:], root: SBody, brain: SBrain, heart: SHeart, caps: [NumCaps]capRule{
		Moving:          {chains: one(SSpine)},
		Sight:           {chains: one(SEyeL, SEyeR)},
		Breathing:       {chains: one(SLung)},
		BloodPumping:    {chains: one(SHeart)},
		BloodFiltration: {chains: one(SKidneyL, SKidneyR), mul: []Part{SLiver}},
		Digestion:       {chains: one(SStomach), mul: []Part{SLiver}},
	}},
	Lizard: {defs: lizardDefs[:], root: LTorso, brain: LBrain, heart: LHeart, caps: [NumCaps]capRule{
		Moving:          {chains: [][]Part{{LLegFL, LFootFL}, {LLegFR, LFootFR}, {LLegHL, LFootHL}, {LLegHR, LFootHR}}, mul: []Part{LSpine}},
		Sight:           {chains: one(LEyeL, LEyeR)},
		Breathing:       {chains: one(LLungL, LLungR)},
		BloodPumping:    {chains: one(LHeart)},
		BloodFiltration: {chains: one(LKidneyL, LKidneyR), mul: []Part{LLiver}},
		Digestion:       {chains: one(LStomach), mul: []Part{LLiver}},
	}},
	Insect: {defs: insectDefs[:], root: IThorax, brain: IBrain, heart: IHeart, caps: [NumCaps]capRule{
		Moving:       {chains: one(ILegFL, ILegFR, ILegML, ILegMR, ILegHL, ILegHR), mul: []Part{INerveCord}},
		Sight:        {chains: one(IEyeL, IEyeR)},
		Hearing:      {chains: one(IAntennaL, IAntennaR)},
		BloodPumping: {chains: one(IHeart)},
		Digestion:    {chains: one(IStomach)},
	}},
}

// planOf is anatomy a's plan (the humanlike one for an unknown value).
func planOf(a Anatomy) *plan {
	if a >= NumAnatomies {
		return &plans[Humanlike]
	}
	return &plans[a]
}

// DefsOf is a copy of anatomy a's part table, at a human's scale.
func DefsOf(a Anatomy) []Def { return append([]Def(nil), planOf(a).defs...) }

// RootOf is anatomy a's root part: the torso, a snake's body, an
// insect's thorax.
func RootOf(a Anatomy) Part { return planOf(a).root }

// table is the body's part table.
func (b *Body) table() []Def {
	if b == nil || b.Anatomy == Humanlike {
		return defs[:]
	}
	return planOf(b.Anatomy).defs
}

// numParts is how many parts the body's plan has.
func (b *Body) numParts() Part { return Part(len(b.table())) }

// PartCount is how many parts the body's plan has.
func (b *Body) PartCount() int { return len(b.table()) }

// Root is the body's root part (the torso, a snake's body, an insect's
// thorax): where whole-body wounds land.
func (b *Body) Root() Part {
	if b == nil {
		return Torso
	}
	return planOf(b.Anatomy).root
}

// maxHP is p's hit points on this body: its plan's, scaled by Scale
// (rounded, at least 1).
func (b *Body) maxHP(p Part) int {
	hp := b.table()[p].MaxHP
	if b.Scale <= 0 || b.Scale == 1000 {
		return hp
	}
	return max((hp*int(b.Scale)+500)/1000, 1)
}

// PartDef is p's definition on this body, MaxHP scaled (the zero Def for
// a part its plan does not have).
func (b *Body) PartDef(p Part) Def {
	if p >= b.numParts() {
		return Def{}
	}
	d := b.table()[p]
	d.MaxHP = b.maxHP(p)
	return d
}

// PartLabel is p's plain-English name on this body.
func (b *Body) PartLabel(p Part) string {
	if p >= b.numParts() {
		return ""
	}
	return b.table()[p].Label
}

// ruleEff is a plan capacity rule's chain average and its multiplier
// (both permille; 1000 for none).
func (b *Body) ruleEff(r *capRule) (chains, mul int32) {
	chains, mul = 1000, 1000
	if n := len(r.chains); n > 0 {
		var sum int32
		for _, ch := range r.chains {
			v := int32(1000)
			for _, p := range ch {
				v = v * b.eff(p) / 1000
			}
			sum += v
		}
		chains = sum / int32(n)
	}
	for _, p := range r.mul {
		mul = mul * b.eff(p) / 1000
	}
	return chains, mul
}

// planCapacity is Capacity for an animal plan: limbs capped by
// consciousness as the humanlike body's, the spine (mul) after, blood
// pumping weighing on moving; a capacity no part serves stays full.
func (b *Body) planCapacity(c Capacity, look condLookup) int32 {
	switch c {
	case Consciousness:
		return b.consciousness(look)
	case BloodPumping:
		return b.pumping(look)
	case BloodFiltration:
		return b.filtration(look)
	}
	r := &planOf(b.Anatomy).caps[c]
	if r.empty() {
		return b.condCap(c, 1000, look)
	}
	ch, mul := b.ruleEff(r)
	var v int32
	switch c {
	case Moving:
		v = limbs(ch, b.consciousness(look)) * mul / 1000
		v = weighted(v, b.pumping(look), pumpingWeight)
	case Manipulation:
		v = limbs(ch, b.consciousness(look)) * mul / 1000
	case Talking:
		v = ch * mul / 1000 * b.consciousness(look) / 1000
	default:
		v = ch * mul / 1000
	}
	return b.condCap(c, v, look)
}

// planRaw is a plan capacity rule read straight off the parts (pumping
// and filtration, which the conditions then adjust).
func (b *Body) planRaw(c Capacity) int32 {
	r := &planOf(b.Anatomy).caps[c]
	if r.empty() {
		return 1000
	}
	ch, mul := b.ruleEff(r)
	return ch * mul / 1000
}

// PartMissing reports p gone (destroyed, amputated or replaced).
func (b *Body) PartMissing(p Part) bool { return p < b.numParts() && b.isMissing(p) }
