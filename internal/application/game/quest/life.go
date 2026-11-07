package quest

import qd "rimworld-go/internal/domain/quest"

// signalOne hands s to live quest id's kind and applies its verdict.
func (q *Quests) signalOne(id qd.ID, s Signal, now uint64) {
	i, ok := q.find(id)
	if !ok || q.list[i].State.Terminal() {
		return
	}
	k := q.kindOf(q.list[i].Kind)
	if k == nil {
		return
	}
	v := k.Signal(q.ctx(now), &q.list[i], s)
	q.verdict(i, now, v, s.Caravan)
}

// verdict applies kind verdict v to quest i (caravan: the one that
// finished it, for an IntoCaravan reward).
func (q *Quests) verdict(i int, now uint64, v Verdict, caravan uint64) {
	switch {
	case v.Succeeds():
		q.succeed(i, now, caravan)
	case v.Fails():
		q.fail(i, now, v.Reason())
	default:
		q.touched(i)
	}
}

// succeed ends active quest i as a success: the chosen reward is paid
// (OnSuccess, unless already paid) and a success letter posted.
func (q *Quests) succeed(i int, now uint64, caravan uint64) {
	n, err := qd.Succeed(q.list[i], now)
	if err != nil || !n.State.Terminal() {
		q.touched(i)
		return
	}
	q.list[i] = n
	if d := q.defOf(n.Kind); d.RewardAt == qd.OnSuccess {
		pay(q.r, d, &q.list[i], caravan)
	}
	post(q.r.Letters, LetterSucceeded, q.list[i])
	q.ended(i, now)
}

// fail ends quest i for reason r. A failure posts a letter naming r; an
// offer that fails expires and, like every expiry, posts none (Q36). A
// paid reward is never taken back (QST-008).
func (q *Quests) fail(i int, now uint64, r qd.Reason) {
	n, err := qd.Fail(q.list[i], now, r)
	if err != nil || !n.State.Terminal() {
		q.touched(i)
		return
	}
	q.list[i] = n
	if n.State == qd.Failed {
		post(q.r.Letters, LetterFailed, n)
	}
	q.ended(i, now)
}

// expire ends offer i: its accept window ran out. No letter (Q36).
func (q *Quests) expire(i int, now uint64) {
	n, err := qd.Expire(q.list[i], now)
	if err != nil || !n.State.Terminal() {
		q.touched(i)
		return
	}
	q.list[i] = n
	q.ended(i, now)
}

// ended lets the kind clean up after quest i ended, then drops its
// indexes and timer.
func (q *Quests) ended(i int, now uint64) {
	if k := q.kindOf(q.list[i].Kind); k != nil {
		k.Ended(q.ctx(now), &q.list[i])
	}
	q.touched(i)
}

// partDue runs quest i's part timer: the kind reacts, then the part is
// Done unless the kind moved its timer.
func (q *Quests) partDue(i, part int, now uint64) {
	n := &q.list[i]
	if n.State != qd.Active || part < 0 || part >= len(n.Parts) {
		q.touched(i)
		return
	}
	at := n.Parts[part].At
	v := VerdictNone
	if k := q.kindOf(n.Kind); k != nil {
		v = k.PartDue(q.ctx(now), n, part)
	}
	if part < len(n.Parts) && n.Parts[part].At == at {
		n.Parts[part].Done = true
	}
	q.verdict(i, now, v, 0)
}

// accept accepts offer i with reward option choice: the kind's Accepted
// runs, and an OnAccept reward is paid at once (QST-008).
func (q *Quests) accept(i int, now uint64, choice int) error {
	d := q.defOf(q.list[i].Kind)
	n, err := qd.Accept(q.list[i], now, choice, d)
	if err != nil {
		return err
	}
	if n.State != qd.Active {
		return qd.ErrMove
	}
	q.list[i] = n
	if k := q.kindOf(n.Kind); k != nil {
		k.Accepted(q.ctx(now), &q.list[i])
	}
	if d.RewardAt == qd.OnAccept {
		pay(q.r, d, &q.list[i], 0)
	}
	q.touched(i)
	return nil
}

// chosen is the reward option n pays: the chosen one, or the only one of
// a quest that never chose (an AutoAccept kind).
func chosen(n qd.Quest) (qd.Reward, bool) {
	switch {
	case n.Chosen >= 0 && int(n.Chosen) < len(n.Rewards):
		return n.Rewards[n.Chosen], true
	case n.Chosen < 0 && len(n.Rewards) == 1:
		return n.Rewards[0], true
	}
	return qd.Reward{}, false
}

// pay gives quest n's chosen reward once (QST-089..091): items by pod at
// home, into the finishing caravan (IntoCaravan; by pod when there is
// none), or nothing (AtSite: the loot is the reward); goodwill to the
// giver; a pawn walks in and joins. Paid is set and never undone.
func pay(r Roles, d qd.Def, n *qd.Quest, caravan uint64) {
	if n.Paid {
		return
	}
	opt, ok := chosen(*n)
	if !ok {
		return
	}
	if len(opt.Lines) > 0 {
		switch {
		case d.Delivery == qd.AtSite:
		case d.Delivery == qd.IntoCaravan && caravan != 0:
			r.Items.IntoCaravan(caravan, opt.Lines)
		default:
			r.Items.Pod(opt.Lines, n.Seed)
		}
	}
	if opt.Goodwill != 0 && n.Giver > 0 {
		r.Goodwill.Adjust(n.Giver-1, opt.Goodwill)
	}
	if opt.Pawn != 0 {
		r.Pawns.WalkIn(opt.Pawn)
	}
	n.Paid = true
}
