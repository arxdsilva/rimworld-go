package mind

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/pawn/mentalbreak"
	"rimworld-go/internal/domain/pawn/social"
)

func TestFightStartsForBothAndEndsWhenOneGoesDown(t *testing.T) {
	f := newFake(1, 2, 3)
	m := New(f)
	m.track[2] = Track{Target: 1} // 2 was on an insulting spree
	m.StartFight(1, 2)
	if !m.InFight(1) || !m.InFight(2) || f.seized != 2 || f.fightNews != 1 {
		t.Fatal("both fighters enter the fight")
	}
	if _, ok := m.track[2]; ok || m.Target(2) != 0 {
		t.Fatal("the fight replaces the spree")
	}
	if _, until, _ := f.Break(1); until != social.FightMinTicks {
		t.Fatalf("until %d", until)
	}
	if !m.Tick(1, f.jobs[1]) {
		t.Fatal("still fighting")
	}
	f.down[2] = true
	m.Downed(2)
	if m.InFight(1) || m.InFight(2) || len(f.outcomes) != 2 {
		t.Fatalf("a downed brawler ends the fight for both: outcomes %v", f.outcomes)
	}
	m.End(1) // nothing to end
}

func TestFightRecoversAfterTheMinimum(t *testing.T) {
	f := newFake(1, 2)
	m := New(f)
	m.StartFight(2, 1)
	f.rolls = []uint32{999}
	for f.now = 0; f.now < social.FightMinTicks+5*fightRecoverEvery; f.now++ {
		if !m.Tick(1, f.jobs[1]) || !m.Tick(2, f.jobs[2]) {
			t.Fatalf("ended at %d with failing rolls", f.now)
		}
	}
	f.rolls = []uint32{0}
	for ; m.InFight(1); f.now++ {
		m.Tick(1, f.jobs[1])
	}
	if (f.now-1-social.FightMinTicks)%fightRecoverEvery != 0 || m.InFight(2) {
		t.Fatalf("recovered at %d, off the roll period", f.now-1)
	}
}

func TestFightPartnerFallbacksAndUnreachable(t *testing.T) {
	f := newFake(1, 2, 3)
	m := New(f)
	m.StartFight(1, 2)
	delete(f.jobs, 1) // 1 lost its job: 2's brawl names it
	if m.partner(1) != 2 {
		t.Fatalf("partner %d", m.partner(1))
	}
	delete(f.jobs, 2) // both lost theirs: same start tick
	if m.partner(1) != 2 {
		t.Fatalf("orphan partner %d", m.partner(1))
	}
	if !m.Tick(1, job.Record{}) || f.jobs[1].Target != 2 {
		t.Fatal("a fighter without a brawl job gets a fresh one")
	}
	f.unreachable[2] = true
	f.now = repathTicks - 1 // 1's re-path tick: it looks for a way again
	if m.Tick(1, f.jobs[1]) || m.InFight(2) {
		t.Fatal("unreachable: the fight ends")
	}
	f.SetBreak(3, mentalbreak.SocialFight, 0) // nobody to fight
	if m.Tick(3, job.Record{}) {
		t.Fatal("a fighter with no partner stops")
	}
}

func TestBrawlClosesInAndPunchesOnCooldown(t *testing.T) {
	f := newFake(1, 2)
	m := New(f)
	m.StartFight(1, 2)
	if !m.Tick(1, f.jobs[1]) || f.jobs[1].State != job.Goto || f.jobs[1].TX != 10 {
		t.Fatalf("far apart: it paths to its foe, %+v", f.jobs[1])
	}
	m.Brawl(1, f.jobs[1])
	if f.jobs[1].State != job.Goto || len(f.punches) != 0 {
		t.Fatal("out of reach: no punch")
	}
	f.x[1] = 9
	if !m.Tick(1, f.jobs[1]) || f.jobs[1].State != job.Do {
		t.Fatalf("beside its foe it holds its ground, %+v", f.jobs[1])
	}
	j := f.jobs[1]
	j.WorkLeft = 2
	m.Brawl(1, j)
	if f.jobs[1].WorkLeft != 1 || len(f.punches) != 0 {
		t.Fatal("the fist cools down first")
	}
	m.Brawl(1, f.jobs[1])
	if len(f.punches) != 1 || f.jobs[1].WorkLeft != 10 {
		t.Fatalf("punches %v, cooldown %d", f.punches, f.jobs[1].WorkLeft)
	}
	f.peaceful[1] = true
	m.Brawl(1, f.jobs[1])
	if len(f.punches) != 1 {
		t.Fatal("one who can't do violence just stands there")
	}
	f.down[2] = true
	m.Brawl(1, f.jobs[1])
	if m.InFight(1) || m.InFight(2) {
		t.Fatal("a downed foe ends the fight")
	}
	m.Brawl(1, job.Record{Kind: job.Brawl, Target: 2})
	if f.jobs[1].State != job.Complete {
		t.Fatal("a brawl job outside a fight completes")
	}
}
