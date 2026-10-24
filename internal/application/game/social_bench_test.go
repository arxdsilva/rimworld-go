package game

import (
	"testing"

	"rimworld-go/internal/domain/ai/job"
	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/pawn/recreation"
)

// socialCrowd is n registered colonists packed on an open map, past the
// first round of interactions.
func socialCrowd(b *testing.B, n int) *Game {
	b.Helper()
	side := 8
	for side*side < 4*n {
		side += 4
	}
	g := New(Config{Seed: 11, Width: side, Height: side})
	for i := 0; i < n; i++ {
		if _, err := g.SpawnNamedPawn(1+(i*2)%(side-2), 1+((i*2)/(side-2))*2%(side-2), ""); err != nil {
			b.Fatal(err)
		}
	}
	for _, id := range g.pawnIDs {
		g.ensurePerson(id)
	}
	for range 3000 { // every pawn interacts, edges and logs fill
		g.phaseSocial()
		g.clock.Advance(1)
	}
	logs := 0
	for _, id := range g.pawnIDs {
		logs += len(g.soc.logs[id])
	}
	if logs == 0 || len(g.soc.edges) == 0 {
		b.Fatalf("crowd never interacted: %d log lines, %d edges", logs, len(g.soc.edges))
	}
	return g
}

func benchSocialPhase(b *testing.B, n int) {
	g := socialCrowd(b, n)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.phaseSocial()
		g.clock.Advance(1)
	}
}

// BenchmarkSocialPhase50 / 200: one tick of the social phase (checks,
// interactions, mood steps) in a crowd, steady state.
func BenchmarkSocialPhase50(b *testing.B)  { benchSocialPhase(b, 50) }
func BenchmarkSocialPhase200(b *testing.B) { benchSocialPhase(b, 200) }

// §12.4: an idle colonist joins company at a table, relaxes there with a
// chat's joy, and counts as chatting (interactions come sooner).
func TestSocialRelaxJoinsCompany(t *testing.T) {
	g, a, b := socialPair(t)
	buildNow(t, g, construction.Table, item.Wood, construction.South, 10, 5)
	if g.idleSocialRelax(a, 300) {
		t.Fatal("joined an empty table")
	}
	g.positions.Set(b, Position{X: 10, Y: 6})
	g.jobs.Set(b, job.Record{Kind: job.Relax, TX: 10, TY: 6, State: job.Do, WorkLeft: 500})
	if !g.idleSocialRelax(a, 300) {
		t.Fatal("did not join company at the table")
	}
	var reached bool
	for range 200 {
		g.SetNeeds(a, 1<<30, 1<<30)
		g.Step(1)
		if j, _ := g.jobs.Get(a); j.Kind == job.SocialRelax && j.State == job.Do {
			reached = true
			break
		}
	}
	if !reached || !g.chatting(a) {
		t.Fatal("never sat down in company")
	}
	if activityOf(job.SocialRelax) != recreation.Chat {
		t.Fatal("social relaxing is not a chat's joy")
	}
}
