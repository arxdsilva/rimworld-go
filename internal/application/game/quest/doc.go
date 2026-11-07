// Package quest is the quests root (QST-1 step 2,
// .agents/gaps/QUESTS_GAP_ANALYSIS.md): every quest the colony was ever
// offered, sorted by id, a due min-heap of their timers, the offer cadence,
// signal routing through indexes, the player's commands and the read model.
// The quest model itself (states, timers, rules) is internal/domain/quest.
//
// It is a consumer sub-package of internal/application/game
// (.agents/specs/SOLID_GO.md §10): it never imports the game package.
// Everything it needs from the simulation comes through the consumer-owned
// roles in deps.go, which the game implements with small adapter types
// (quest_host.go). Each quest kind is a Kind passed to New by the
// composition root (quest/kinds/, QST-2); there is no registration.
//
// The root is deterministic: every roll comes from the Rolls role (the
// quest stream), time only from the ticks it is given or Clock, maps are
// only looked up (never ranged), and the heap orders ties by quest id.
package quest
