package quest

import (
	qd "rimworld-go/internal/domain/quest"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/mapgen"
)

// Roles is everything the quests root and its kinds need from the
// simulation, one small consumer-owned interface each (SOLID_GO.md §7,
// §10; at most 8 methods). The game implements every role with a small
// adapter type (quest_host.go). Every method takes and returns values.
//
// Faction arguments here are plain indexes (0-based, the game's own);
// quests, signals and commands store a faction as "1 + index, 0 none".
type Roles struct {
	World    World
	Clock    Clock
	Sites    Sites
	Pawns    Pawns
	Items    Items
	Threats  Threats
	Letters  Letters
	Goodwill Goodwill
	Rolls    Rolls
	Builder  Builder
}

// Builder lays a quest's site on its map (QST-104..111). Its calls act on
// the current map, so a kind calls it only while SigSiteOpened is being
// delivered (the site map is current then). Cells are the map's own.
type Builder interface {
	// Size is the current map's width and height in cells.
	Size() (w, h int)
	// Camp lays compound c as a settlement's (pieces, loot forbidden for
	// the pack-up to carry) and spawns one defender of faction f per
	// weapon in arms at its posts, in a defending group that flees at
	// half losses; seed draws (no simulation stream moves). The number of
	// defenders spawned.
	Camp(c mapgen.Compound, f int, arms []item.Kind, seed uint64) int
	// Goods lays lines as stacks around cell (x, y), forbidden when
	// forbid (the pack-up carries forbidden goods too).
	Goods(l []trade.Line, x, y int, forbid bool)
	// Downed spawns the person made from seed (PawnOf) downed and wounded
	// near (x, y), a guest of faction f (-1: none) who stays: its id.
	Downed(seed uint64, f, x, y int) (entity.EntityID, bool)
	// Held spawns the person made from seed near (x, y) as a prisoner of
	// faction f: a guest who stays put until freed (OrderFreePrisoner,
	// SigPawnFreed). Its id.
	Held(seed uint64, f, x, y int) (entity.EntityID, bool)
}

// World is the colony and the planet as the generator sees them.
type World interface {
	// Landed is the landing tick: windows count from it (QST-020).
	Landed() uint64
	// Colonists is the colony's colonist count, caravans included
	// (QST-036).
	Colonists() int
	// Points are the raid points summed over every colony map and caravan,
	// one difficulty.ThreatPoints call (QST-025).
	Points() int
	// ThreatScalePct and RewardPct are the difficulty's threat scale and
	// quest reward percent (QST-034, QST-035).
	ThreatScalePct() int
	RewardPct() int
	// QuestThreats: quests may carry violent threats (QST-033).
	QuestThreats() bool
	// Factions is every faction, by index.
	Factions() []FactionView
	// NearTile is a free world tile min..max tiles from home, picked from
	// seed (ok false: none).
	NearTile(seed uint64, min, max int) (tile int, ok bool)
}

// FactionView is one faction as kinds pick givers and targets (QST-038).
type FactionView struct {
	Index         int
	Relation      faction.Relation
	Goodwill      int
	CanBeFriendly bool
	Defeated      bool
	Besieged      bool
	// Settlements are its settlement tiles, ascending.
	Settlements []int
}

// Clock is the simulation tick, for commands (Apply) and Offer.
type Clock interface {
	Now() uint64
}

// Site is a quest site to place on a world tile.
type Site struct {
	Quest   qd.ID
	Kind    qd.KindID
	Tile    int
	Layout  uint8
	Seed    uint64
	Faction int // the site's owner, 1 + index (0: none)
}

// Sites are the world objects quests put on the planet (QST-100..119).
type Sites interface {
	// Place puts site s on its tile; false when the tile cannot take it.
	Place(s Site) bool
	// Remove takes quest id's site off tile (nothing when there is none).
	Remove(tile int, id qd.ID)
	// Settlement is one of faction f's settlement tiles, picked from seed.
	Settlement(f int, seed uint64) (tile int, ok bool)
	// Deposit is the long-range scanner's mineral site on tile: its lump
	// as one line (ok false: none there, QST-116).
	Deposit(tile int) (lump trade.Line, ok bool)
}

// PawnPreview is what the window shows of a pawn reward or a joiner before
// it exists (QST-087): the same seed spawns the same pawn.
type PawnPreview struct {
	Name      string
	Age       int
	Backstory string
}

// Pawns makes, joins and reads the pawns quests are about.
type Pawns interface {
	Preview(seed uint64) PawnPreview
	// WalkIn spawns the pawn of seed at the home map's edge and has it
	// join the colony.
	WalkIn(seed uint64) (entity.EntityID, bool)
	// Join takes an existing pawn into the colony.
	Join(id entity.EntityID) bool
	Alive(id entity.EntityID) bool
	// BestSocial is caravan's member with the best Social skill.
	BestSocial(caravan uint64) (id entity.EntityID, level int, ok bool)
	GrantXP(id entity.EntityID, s skill.ID, xp int)
}

// Items moves goods for rewards and requests.
type Items interface {
	// Pod drops lines by pod at the home colony (QST-089); seed picks the
	// spot.
	Pod(lines []trade.Line, seed uint64) bool
	// IntoCaravan adds lines to caravan's cargo (QST-090).
	IntoCaravan(caravan uint64, lines []trade.Line) bool
	// Has reports whether caravan carries lines; Take removes them.
	// Only goods of normal quality or better and not tainted count
	// (QST-071).
	Has(caravan uint64, lines []trade.Line) bool
	Take(caravan uint64, lines []trade.Line) bool
	// CaravanAt is the world tile caravan stands on, arrived or visiting
	// (ok false: no such caravan, or it is moving).
	CaravanAt(caravan uint64) (tile int, ok bool)
}

// Threats starts the threats quests send.
type Threats interface {
	// Raid sends faction f (index) at the home colony with points.
	Raid(f, points int, seed uint64) bool
	// Manhunters sends a manhunter pack with points at the home colony.
	Manhunters(points int, seed uint64) bool
	// Condition starts game condition kind (CondHeatWave, ...) at the home
	// colony for ticks.
	Condition(kind uint8, ticks uint64) bool
	// Ambush attacks caravan with faction f (index) at once.
	Ambush(caravan uint64, f, points int, seed uint64) bool
}

// Letters posts quest letters (QST-131..134).
type Letters interface {
	Post(l Letter)
}

// Goodwill is the factions' standing with the colony.
type Goodwill interface {
	// Adjust changes faction f's goodwill by delta (faction.Adjust).
	Adjust(f, delta int)
	// Relation is faction f's relation now; Count how many factions there
	// are. The hourly snapshot reads them (QST-012).
	Relation(f int) faction.Relation
	Count() int
}

// Rolls is the quest stream (streamQuest, QST-147): every quest roll.
type Rolls interface {
	// Intn is a roll in [0, n); n > 0.
	Intn(n int) int
	// Seed is a fresh 64-bit seed for a quest or part.
	Seed() uint64
}

// The game conditions a quest may force (Threats.Condition, QST-042).
const (
	CondNone uint8 = iota
	CondHeatWave
	CondColdSnap
	CondVolcanicWinter
)
