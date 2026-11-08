package windows

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/ports"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/quest"
	"rimworld-go/internal/application/game/setupview"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	qd "rimworld-go/internal/domain/quest"
	"rimworld-go/internal/domain/sim/clock"
)

// The Quests window's words (QST-125..130): our own English, each literal
// inside a T/Tf call so the catalog extractor sees it. The quest only
// hands keys and typed arguments (quest.Line); this file turns them into
// text.

// questTexts is what the words need of the shell.
type questTexts interface {
	ports.Texts
	FactionName(ref int) string
}

// questTitle is a quest's name in the list and the detail: its kind's
// game-side name (quest.KindName), else "Quest <id>".
func questTitle(tr ports.Texts, q game.QuestView) string {
	if n := quest.KindName(q.Kind); n != "" {
		return tr.T(n)
	}
	return tr.Tf("Quest %d", q.ID)
}

// questSpan is a span of ticks as the detail says it: "5.6 days" from a
// day, else "7 hours" (at least 1).
func questSpan(tr ports.Texts, ticks uint64) string {
	if ticks >= clock.TicksPerDay {
		return tr.Tf("%.1f days", float64(ticks)/clock.TicksPerDay)
	}
	return tr.Tf("%d hours", max(1, ceilHours(ticks)))
}

// questShort is a row's time cell: "6d" from a day, "22h", "0h" for none.
func questShort(tr ports.Texts, ticks uint64) string {
	if ticks >= clock.TicksPerDay {
		return tr.Tf("%dd", ticks/clock.TicksPerDay)
	}
	return tr.Tf("%dh", ceilHours(ticks))
}

// ceilHours is ticks in hours rounded up, as the HUD's expiring alert
// counts them: 6.2 hours left reads 7.
func ceilHours(ticks uint64) uint64 { return (ticks + clock.TicksPerHour - 1) / clock.TicksPerHour }

// questLeft is the time a quest's running timer has left at now, and
// whether one runs: an offer's accept-by, an active quest's deadline.
// Ended quests have none.
func questLeft(q game.QuestView, now uint64) (uint64, bool) {
	var at uint64
	switch q.State {
	case qd.Offered:
		at = q.AcceptBy
	case qd.Active:
		at = q.Deadline
	}
	if at == 0 {
		return 0, false
	}
	if at <= now {
		return 0, true
	}
	return at - now, true
}

// questUrgent: an offer with under a day to accept (the row's time turns
// to the danger role).
func questUrgent(q game.QuestView, now uint64) bool {
	left, ok := questLeft(q, now)
	return ok && q.State == qd.Offered && left < clock.TicksPerDay
}

// questTimeLines are the detail's lines under the title (QST-126).
func questTimeLines(tr ports.Texts, q game.QuestView, now uint64) []string {
	ago := func(t uint64) string { return questSpan(tr, now-min(now, t)) }
	var out []string
	switch q.State {
	case qd.Offered:
		if left, ok := questLeft(q, now); ok {
			out = append(out, tr.Tf("Offer ends in %s", questSpan(tr, left)))
		}
	case qd.Active:
		out = append(out, tr.Tf("Accepted %s ago", ago(q.AcceptedAt)))
		if left, ok := questLeft(q, now); ok {
			out = append(out, tr.Tf("Time left: %s", questSpan(tr, left)))
		}
	case qd.Succeeded:
		out = append(out, tr.Tf("Completed %s ago", ago(q.EndedAt)))
	case qd.Failed:
		out = append(out, tr.Tf("Failed %s ago: %s", ago(q.EndedAt), tr.T(quest.ReasonText(q.Reason))))
	case qd.Expired:
		out = append(out, tr.Tf("Lapsed %s ago", ago(q.EndedAt)))
	}
	if q.Dismissed {
		out = append(out, tr.T("Dismissed"))
	}
	return out
}

// questArg is one detail argument as text, by its kind.
func questArg(tr questTexts, a quest.Arg, now uint64) string {
	switch a.Kind {
	case quest.ArgText:
		return a.S
	case quest.ArgFaction:
		if n := tr.FactionName(int(a.N)); n != "" {
			return n
		}
		return tr.T("an unknown faction")
	case quest.ArgTick:
		if t := uint64(max(0, a.N)); t > now {
			return tr.Tf("in %s", questSpan(tr, t-now))
		}
		return tr.Tf("on %s", historyDate(uint64(max(0, a.N))))
	case quest.ArgTile:
		return tr.Tf("tile %d", a.N)
	case quest.ArgItem:
		return setupview.ItemLabel(item.Kind(a.N))
	}
	return strconv.FormatInt(a.N, 10)
}

// questLine is one detail line in our words. An unknown key still shows:
// the key and its arguments.
func questLine(tr questTexts, l quest.Line, now uint64) string {
	a := make([]any, len(l.Args))
	for i, x := range l.Args {
		a[i] = questArg(tr, x, now)
	}
	if s, ok := questLineKnown(tr, l.Key, a); ok {
		return s
	}
	parts := []string{l.Key}
	for _, x := range a {
		parts = append(parts, x.(string))
	}
	return strings.Join(parts, " ")
}

// questLineKnown formats the keys the kinds emit; ok is false for any other
// key (a missing argument shows empty).
func questLineKnown(tr ports.Texts, key string, a []any) (string, bool) {
	switch key {
	case "quest.camp.giver":
		return tr.Tf("%s wants an outlaw camp broken up.", arg(a, 0)), true
	case "quest.camp.target":
		return tr.Tf("The camp's people answer to %s.", arg(a, 0)), true
	case "quest.camp.site":
		return tr.Tf("It lies at %s.", arg(a, 0)), true
	case "quest.camp.deadline":
		return tr.Tf("The outlaws break camp %s.", arg(a, 0)), true
	case "quest.camp.standing":
		return tr.Tf("Guards still standing: %s of %s.", arg(a, 0), arg(a, 1)), true
	case "quest.joiner.about":
		return tr.Tf("A traveller from %s asks to live with us.", arg(a, 0)), true
	case "quest.joiner.warning":
		return tr.T("Someone may be after them."), true
	case "quest.joiner.arrived":
		return tr.T("They came, and what was after them came too."), true
	case "quest.joiner.coming":
		return tr.T("They are on their way to the colony."), true
	case "quest.orelump.lump":
		return tr.Tf("A seam of %s lies near the surface, about %s units.", arg(a, 0), arg(a, 1)), true
	case "quest.orelump.reach":
		return tr.Tf("Send a caravan with miners to %s.", arg(a, 0)), true
	case "quest.refugee.site":
		return tr.Tf("A survivor is hiding at %s.", arg(a, 0)), true
	case "quest.refugee.deadline":
		return tr.Tf("Their hiding place will be found %s.", arg(a, 0)), true
	case "quest.refugee.pawn":
		return tr.Tf("%s, %s years old, asks for shelter.", arg(a, 0), arg(a, 1)), true
	case "quest.refugee.helped":
		return tr.T("They reached safety with us."), true
	case "quest.refugee.waiting":
		return tr.T("They are waiting for someone to fetch them."), true
	case "quest.refugee.guards":
		return tr.T("Hunters keep watch near the hiding place."), true
	case "quest.request.item":
		return tr.Tf("They need %s.", arg(a, 0)), true
	case "quest.request.count":
		return tr.Tf("Amount wanted: %s.", arg(a, 0)), true
	case "quest.request.tile":
		return tr.Tf("Deliver it by caravan to %s.", arg(a, 0)), true
	case "quest.request.giver":
		return tr.Tf("The request comes from %s.", arg(a, 0)), true
	case "quest.rescue.site":
		return tr.Tf("Someone is locked up at %s, held by %s.", arg(a, 0), arg(a, 1)), true
	case "quest.rescue.deadline":
		return tr.Tf("The captors move their prisoner %s.", arg(a, 0)), true
	case "quest.rescue.pawn":
		return tr.Tf("The prisoner is called %s.", arg(a, 0)), true
	case "quest.rescue.calling":
		return tr.T("They are calling for help."), true
	case "quest.rescue.freed":
		return tr.T("The prisoner walks free."), true
	case "quest.rescue.held":
		return tr.T("Our people have reached the prisoner."), true
	case "quest.stash.site":
		return tr.Tf("A guarded cache waits at %s.", arg(a, 0)), true
	case "quest.stash.deadline":
		return tr.Tf("The guards carry it off %s.", arg(a, 0)), true
	case "quest.stash.item":
		return tr.Tf("Inside: %s, %s of them.", arg(a, 0), arg(a, 1)), true
	case "quest.talks.host":
		return tr.Tf("%s offers to talk peace.", arg(a, 0)), true
	case "quest.talks.site":
		return tr.Tf("The meeting is at %s.", arg(a, 0)), true
	case "quest.talks.nosite":
		return tr.T("The meeting place no longer exists."), true
	case "quest.talks.caravan":
		return tr.Tf("Send a caravan with a good talker to %s.", arg(a, 0)), true
	case "quest.talks.triumph":
		return tr.Tf("The talks were a great success (goodwill %s).", arg(a, 0)), true
	case "quest.talks.success":
		return tr.Tf("The talks went well (goodwill %s).", arg(a, 0)), true
	case "quest.talks.flounder":
		return tr.Tf("The talks led nowhere (goodwill %s).", arg(a, 0)), true
	case "quest.talks.backfire":
		return tr.Tf("The talks went badly (goodwill %s).", arg(a, 0)), true
	case "quest.talks.disaster":
		return tr.Tf("The talks ended in insults (goodwill %s).", arg(a, 0)), true
	case "quest.talks.gift":
		return tr.Tf("They sent a gift worth %s.", arg(a, 0)), true
	}
	return "", false
}

// arg is a[i], or "" past its end (questLineKnown checks the count first).
func arg(a []any, i int) any {
	if i < len(a) {
		return a[i]
	}
	return ""
}

// rewardChips are an option's chips as text, in order: the items, the
// goodwill, the pawn, the site's loot; tips holds each chip's hover card
// (the pawn's preview, QST-137).
func rewardChips(tr questTexts, giver int, r quest.RewardView) (labels, tips []string) {
	for _, l := range r.Lines {
		labels, tips = append(labels, rewardItem(tr, l)), append(tips, "")
	}
	if r.Goodwill != 0 {
		name := tr.FactionName(giver)
		labels, tips = append(labels, tr.Tf("Goodwill %+d", r.Goodwill)), append(tips, name)
	}
	if r.HasPawn {
		p := r.Pawn
		tip := tr.Tf("%s, %d\n%s", p.Name, p.Age, p.Backstory)
		labels, tips = append(labels, tr.Tf("%s joins", p.Name)), append(tips, tip)
	}
	if r.SiteLoot {
		labels, tips = append(labels, tr.T("Loot at the site")), append(tips, tr.T("Loot at the site\nWhatever the site holds is ours to take."))
	}
	return labels, tips
}

// rewardItem is an item chip: "Wood x80".
func rewardItem(tr ports.Texts, l trade.Line) string {
	return tr.Tf("%s x%d", setupview.ItemLabel(l.Kind), l.Qty)
}
