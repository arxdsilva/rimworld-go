package hud

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/quest"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
)

// "Quest expires soon" (QST-134): an offer nobody has taken has under a day
// left before its AcceptBy. The click opens the Quests window on it.

// QuestExpiringKey is the alert's key prefix: "questexpiring.<quest id>".
const QuestExpiringKey = "questexpiring"

// QuestExpiringLeft is how long an offer shows the alert: a pure function of
// AcceptBy and now, so it cannot flap. Time only moves forward and an offer
// only leaves Offered once, so the alert comes on once at the threshold and
// goes off once when the offer is taken or lapses.
func QuestExpiringLeft(acceptBy, now uint64) (left uint64, ok bool) {
	if acceptBy <= now || acceptBy-now >= clock.TicksPerDay {
		return 0, false
	}
	return acceptBy - now, true
}

// QuestExpiringAlertIn is the alert for the soonest-expiring offer, if any.
func QuestExpiringAlertIn(tr lang.Tr, w game.WorldView, now uint64) (Alert, bool) {
	var best game.QuestView
	var bestLeft uint64
	for _, q := range w.Quests {
		if q.State != qd.Offered {
			continue
		}
		if left, ok := QuestExpiringLeft(q.AcceptBy, now); ok && (bestLeft == 0 || left < bestLeft) {
			best, bestLeft = q, left
		}
	}
	if bestLeft == 0 {
		return Alert{}, false
	}
	hours := int((bestLeft + clock.TicksPerHour - 1) / clock.TicksPerHour) // rounded up: never "0 hours"
	return Alert{
		Key:   QuestExpiringKey + "." + strconv.FormatUint(uint64(best.ID), 10),
		Label: lang.Tf(tr, "Quest expires in %d h: %s", hours, tr.T(quest.KindName(best.Kind))),
	}, true
}

// showQuest opens the Quests window on quest id (QST-3). A free function,
// so the Right and Alerts types do not grow.
func showQuest(sh any, id uint32) {
	if d, ok := sh.(ports.Dispatcher); ok {
		d.Do(ports.ShowQuest{ID: id})
	}
}

// questOfAlertKey is the quest id in a "questexpiring.<id>" key.
func questOfAlertKey(key string) (uint32, bool) {
	rest, ok := strings.CutPrefix(key, QuestExpiringKey+".")
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(rest, 10, 32)
	return uint32(n), err == nil
}
