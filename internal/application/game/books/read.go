package books

import (
	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/ai/pathing"
	"rimworld-go/internal/domain/economy/book"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/quality"
	"rimworld-go/internal/domain/pawn/recreation"
	"rimworld-go/internal/domain/pawn/skill"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/entity"
)

// Reading (wiki: Books): a colonist walks to a book, picks it up, sits
// down with it (the best free seat near the book, a room with a reading
// bonus preferred; standing where the book lay when there is none) and
// reads. The job is job.Read: Count 0 while it fetches the book, 1 while
// it reads; WorkLeft counts the reading down, so the job carries its own
// progress and nothing else is saved. When it ends the book is put down
// where the reader is (the game's dropCarry) and haulers shelve it.
//
// Hysteresis: free time picks a book only below recreation.Entertained,
// and a free-time read lasts ReadTicks or until the need is full, so a
// reader never flips between reading and something else.

const (
	// ReadTicks is one reading: an hour (wiki: the read order forces an
	// hour; a free-time read lasts as long, original tuning).
	ReadTicks = clock.TicksPerHour
	// SeekReach is how far (tiles) a colonist goes for a book, SeatReach
	// how far from the book it looks for a seat (original tuning).
	SeekReach = 40
	SeatReach = 12
	// seatBonusStep: a seat in a room this much more reading bonus
	// (permille) is worth one tile further (original tuning).
	seatBonusStep = 20
	// seekOdds: a bored colonist with a book in reach reads on one free
	// time in this many (original tuning).
	seekOdds = 2
)

// Reasons an order to read is refused.
const (
	ReasonNotBook = "not a book"
	ReasonBlind   = "can't see"
	ReasonInUse   = "in use"
)

// Seek starts free-time reading for id: a random loose book within
// SeekReach that it may take. It reports whether reading began.
func (b *Books) Seek(id entity.EntityID) bool {
	if b.m.Joy(id) >= recreation.Entertained || !b.m.Sighted(id) {
		return false
	}
	px, py := b.hd.Pos(id)
	b.cands = b.cands[:0]
	for _, bid := range b.looseBooks() {
		st := b.it.StackRef(bid)
		if st == nil || st.Carrier != 0 || st.Forbidden || st.Unclaimed || pathing.Dist(px, py, st.X, st.Y) > SeekReach ||
			!b.c.Allowed(id, st.X, st.Y) || !b.hd.Free(id, bid, job.Read) {
			continue
		}
		b.cands = append(b.cands, bid)
	}
	if len(b.cands) == 0 || b.c.Roll(seekOdds) != 0 {
		return false // a book is one fun among several: read on one free time in seekOdds
	}
	rec, why := b.Order(id, b.cands[b.c.Roll(len(b.cands))])
	if why != "" || !b.hd.StartJob(id, rec) {
		return false
	}
	b.hd.Claim(id, rec.Target)
	return true
}

// Order is the read job for book on id's order (the float menu's "Read"),
// or why not.
func (b *Books) Order(id, bk entity.EntityID) (job.Record, string) {
	st := b.it.StackRef(bk)
	switch {
	case st == nil || !st.Kind.IsBook():
		return job.Record{}, ReasonNotBook
	case !b.m.Sighted(id):
		return job.Record{}, ReasonBlind
	case st.Carrier != 0 && st.Carrier != id || !b.hd.Free(id, bk, job.Read):
		return job.Record{}, ReasonInUse
	}
	sx, sy, ok := b.hd.Spot(id, st.X, st.Y)
	if !ok {
		return job.Record{}, ReasonInUse
	}
	return job.Record{Kind: job.Read, Target: bk, TX: sx, TY: sy, WorkLeft: ReadTicks}, ""
}

// Do runs one tick of a read job.
func (b *Books) Do(id entity.EntityID, j job.Record) {
	if j.Count == 0 {
		b.pickUp(id, j)
		return
	}
	st := b.it.StackRef(j.Carry)
	if st == nil || st.Carrier != id || !st.Kind.IsBook() {
		j.State, j.Fail = job.Fail, job.TargetGone
		b.hd.SetJob(id, j)
		return
	}
	b.read(id, j.WorkLeft, *st)
	if j.WorkLeft--; j.WorkLeft <= 0 || !j.Forced && b.m.Joy(id) >= recreation.Full {
		j.State = job.Complete
	}
	b.hd.SetJob(id, j)
}

// pickUp takes the book and heads for a seat.
func (b *Books) pickUp(id entity.EntityID, j job.Record) {
	st := b.it.StackRef(j.Target)
	if st == nil || !st.Kind.IsBook() || st.Carrier != 0 && st.Carrier != id || !b.hd.Free(id, j.Target, job.Read) {
		j.State, j.Fail = job.Fail, job.TargetGone
		b.hd.SetJob(id, j)
		return
	}
	got := *st
	got.Carrier = id
	b.it.SetStack(j.Target, got)
	j.Carry, j.Count = j.Target, 1
	if x, y, ok := b.seat(id, got.X, got.Y); ok && b.c.ClaimTile(id, x, y) && b.hd.Walk(id, j, x, y) {
		return
	}
	b.hd.SetJob(id, j) // no seat in reach: read standing here
}

// seat is the best free seat cell within SeatReach of (bx, by): nearest,
// a tile further for every seatBonusStep of reading bonus.
func (b *Books) seat(id entity.EntityID, bx, by int) (int, int, bool) {
	b.structure()
	best, bx2, by2 := 1<<30, 0, 0
	for _, c := range b.seats {
		d := pathing.Dist(bx, by, c[0], c[1])
		if d > SeatReach || b.it.TileTaken(c[0], c[1]) || !b.c.Allowed(id, c[0], c[1]) {
			continue
		}
		if s := d - b.BonusAt(c[0], c[1])/seatBonusStep; s < best {
			best, bx2, by2 = s, c[0], c[1]
		}
	}
	return bx2, by2, best < 1<<30
}

// read is one reading tick with left ticks to go (book st): recreation by
// the book's power, Intellectual for any book, a textbook's skills and a
// schematic's projects, each per hour spread evenly over the reading
// (the tick's share of the hourly rate, exact over the hour), all but the
// Intellectual times the room's reading bonus.
func (b *Books) read(id entity.EntityID, left int, st item.Stack) {
	e := ReadTicks - left + 1 // this tick's place in the reading, 1..ReadTicks
	px, py := b.hd.Pos(id)
	bonus := 1000 + b.BonusAt(px, py)
	q, _ := quality.FromStored(st.Quality)
	joy := book.JoyPerHour * book.Power(st.Kind, q) / 1000 * bonus / 1000
	b.m.SetJoy(id, min(recreation.Full, b.m.Joy(id)+share(joy, e)))
	b.m.Learn(id, skill.Intellectual, share(book.IntellectualXP, e))
	sks := book.Skills(st.Kind, st.Book)
	for _, sk := range sks {
		if b.m.Level(id, sk) < book.Cap(q) {
			b.m.Learn(id, sk, share(book.SkillXP(q, len(sks))*bonus/1000, e))
		}
	}
	ps := book.Projects(st.Kind, st.Book)
	for _, p := range ps {
		if n := share(book.Points(q, len(ps))*bonus/10/max(1, b.m.Cost(p)), e); n > 0 { // a dearer project takes longer, as at a bench
			b.m.Study(p, n)
		}
	}
}

// share is tick e's part of rate an hour: the whole rate over ReadTicks
// ticks, no fraction lost.
func share(rate, e int) int { return rate*e/ReadTicks - rate*(e-1)/ReadTicks }
