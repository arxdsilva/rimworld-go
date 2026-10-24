package social

// MemKind is an opinion memory (append only, saved).
type MemKind uint8

const (
	MemChitchat MemKind = iota
	MemDeepTalk
	MemSlighted
	MemInsulted
	MemKindWords
	MemRebuffedMe
	MemFailedRomance
	MemCheatedOnMe
	MemRejectedMyProposal
	MemIRejectedProposal
	MemBrokeUpWithMe
	MemDivorcedMe
	MemAngeringFight
	MemCatharticFight
	MemHarmedMe
	MemKilledMyParentChild
	MemKilledMySibling
	MemKilledMySpouse
	MemKilledMyLover
	MemKilledMyKin
	MemRecruitedMe
	MemRescuedMe
	MemRescuedMeStranger
	MemBotchedSurgery
	NumMemKinds
)

// OpinionMemory is one remembered event shaping an opinion of Subject.
type OpinionMemory struct {
	Kind    MemKind
	Subject ID
	Expires uint64
}

// AddMemory remembers k about subject at tick now, stacking per its def.
func AddMemory(ms []OpinionMemory, k MemKind, subject ID, now uint64) []OpinionMemory {
	return ms // TODO(M3)
}

// PruneMemories drops expired memories.
func PruneMemories(ms []OpinionMemory, now uint64) []OpinionMemory {
	return ms // TODO(M3)
}
