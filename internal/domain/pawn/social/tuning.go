// Values follow the published RimWorld wiki by explicit user decision (ENG-003 balance override).

package social

import "rimworld-go/internal/domain/sim/clock"

// Time units, in ticks.
const (
	Hour = clock.TicksPerHour
	Day  = clock.TicksPerDay
)

// Opinion bounds and bands (centi-points).
const (
	OpinionMax = 10000
	OpinionMin = -10000
	// Hysteresis: friend enters at >=20 and exits below 15; rival enters at
	// <=-20 and exits above -15.
	FriendEnter = 2000
	FriendExit  = 1500
	RivalEnter  = -2000
	RivalExit   = -1500
)

// Opinion term values (centi-points).
const (
	BeautyStep         = 2000 // per beauty level
	BeautyCap          = 4000
	DisfiguredPenalty  = -1500
	VoicePenalty       = -2500 // grating voice, rasping breath
	DistrustPenalty    = -2500
	TirelessUnhurried  = -2000
	TirelessIdle       = -3000
	AbstainerAddicted  = -2500
	AbstainerIndulgent = -2000
	AbstainerHedonist  = -3000
	AugmentFanPerPart  = 800
	AugmentFanCap      = 4000
	PuristPerPart      = -800
	PuristCap          = -4000
)

// Relation opinion bonuses (centi-points).
const (
	bonusParent       = 3000
	bonusChild        = 3000
	bonusSibling      = 2000
	bonusHalfSibling  = 1500
	bonusGrandparent  = 1500
	bonusGrandchild   = 1500
	bonusGreatGrand   = 500
	bonusAuntUncle    = 1000
	bonusNieceNephew  = 1000
	bonusGrandAunt    = 500
	bonusCousin       = 1000
	bonusSecondCousin = 500
	bonusKin          = 500
	bonusStep         = 500
	bonusInLaw        = 500
	bonusLover        = 3500
	bonusFiance       = 3000
	bonusSpouse       = 3000
	bonusExLover      = -1500
	bonusExSpouse     = -1500
)

// Memory values (centi-points) and durations.
const (
	memChitchat        = 66
	memDeepTalk        = 1500
	memSlighted        = -500
	memInsulted        = -1500
	memKindWords       = 1500
	memRebuffedMe      = -1000
	memFailedRomance   = -1500
	memCheatedOnMe     = -7000
	memRejectedMyProp  = -3000
	memIRejectedProp   = -1500
	memBrokeUpWithMe   = -5000
	memDivorcedMe      = -7000
	memAngeringFight   = -2200
	memCatharticFight  = 3800
	memHarmedMe        = -1500
	memKilledParent    = -8000
	memKilledSibling   = -8000
	memKilledSpouse    = -6500
	memKilledLover     = -5000
	memKilledKin       = -1500
	memRecruitedMe     = 2000
	memRescuedMe       = 1500
	memRescuedStranger = 2500
	memBotchedSurgery  = -2000

	ChitchatCap = 1000 // total chitchat memory cap, +10 points

	stackFactorDefault = 1000
	stackFactorSocial  = 900
)

// Interaction base weights (permille).
const (
	WeightChitchat  = 1000
	WeightDeepTalk  = 75
	WeightSlight    = 20
	WeightInsult    = 7
	WeightKindWords = 10
	WeightRomance   = 100
	WeightBreakup   = 100
	WeightProposal  = 50
	AffairFactor    = 250 // romance attempt weight when the initiator is partnered
)

// Social impact stat.
const (
	ImpactBase  = 820 // permille at skill 0
	ImpactFloor = 200
)

// Romance.
const (
	MinRomanceAge       = 16
	BreakupGrace        = 5 * Day
	ProposalCooldown    = 10 * Day
	RebuffCooldown      = 3 * Day
	PartneredRomanceCut = 250 // permille of the success chance when recipient is partnered
	RomanceSuccessMax   = 950
	AffairBreakupBonus  = 300
)

// Gatherings.
const (
	WeddingGatherMin  = 2 * Hour
	WeddingCeremony   = 1000
	WeddingReception  = 3 * Hour
	PartyDuration     = 3 * Hour
	GatherRetryAfter  = Day
	FightMinTicks     = 420
	FightPairCooldown = Day
)

// Fight chances (permille) and multipliers (permille).
const (
	FightChanceInsult = 40
	FightChanceSlight = 5
	BloodthirstyMult  = 4000
	FightChanceMax    = 1000
)

// Fight outcomes: mood points and days.
const (
	FightAngeringMood  = -22
	FightCatharticMood = 38
	FightOutcomeDays   = 20
	FightAngeringShare = 500 // permille of fights that anger
	grievanceDays      = 30
	griefFriendMood    = -10
	griefFriendDays    = 20
	griefRivalMood     = 10
	griefRivalDays     = 10
	lostDays           = 15
	lostFriendDays     = 10
	killerSpouseLover  = -6500
	killerChildSibling = -8000
	killerLover        = -5000
	killerDistant      = -1500
)

// Family walk depth.
const MaxFamilyDepth = 4

// Compatibility: Irwin-Hall(4) scaled to sigma 1000 permille, plus age term.
const (
	compatAgeMax    = 450
	compatAgeStep   = 100
	compatAgeCap    = 2000
	irwinSum        = 4 * 65535
	irwinMeanTwice  = irwinSum // mean*2
	irwinSigmaTimes = 37837    // sigma of the 4x16-bit sum
)
