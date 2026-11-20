// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package starlarkhost

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"rimworld-go/internal/application/scripting"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/testkit/allocs"
)

type fakePawns struct{ ps []scripting.Pawn }

func (f fakePawns) Pawns() []scripting.Pawn { return f.ps }
func (f fakePawns) Pawn(id uint64) (scripting.Pawn, bool) {
	for _, p := range f.ps {
		if p.ID == id {
			return p, true
		}
	}
	return scripting.Pawn{}, false
}

func colonists() []scripting.Pawn {
	return []scripting.Pawn{
		{ID: 3, Name: "Ada", X: 1, Y: 2, Food: 40, Rest: 50, Mood: 60, Downed: false, Drafted: true},
		{ID: 9, Name: "Bo", X: 7, Y: 8, Food: 10, Rest: 20, Mood: 30, Downed: true, Drafted: false},
		{ID: 12, Name: "Cy", X: 4, Y: 5, Food: 100, Rest: 0, Mood: 99},
	}
}

func fireWith(h *Host, hook scripting.Hook, tick uint64, p scripting.PawnReader, due []scripting.Pawn) scripting.Result {
	return h.Fire(scripting.Event{Hook: hook, Tick: tick, Seed: 7, Pawns: due}, &fakeWorld{}, p)
}

func msgs(r scripting.Result) []string {
	var out []string
	for _, c := range r.Commands {
		if c.Kind == command.ScriptMessage {
			out = append(out, c.Name)
		}
	}
	return out
}

func TestCtxPawnsBatched(t *testing.T) {
	t.Parallel()
	src := `def on_hour(ctx):
    for p in ctx.pawns():
        ctx.message("%d|%s|%d|%d|%d|%d|%d|%s|%s" % (p.id, p.name, p.x, p.y, p.food, p.rest, p.mood, p.downed, p.drafted))
`
	h := New([]Plugin{plugin("p", "scripts/s.star", src)})
	got := msgs(fireWith(h, scripting.OnHour, 1, fakePawns{colonists()}, nil))
	var want []string
	for _, p := range colonists() {
		want = append(want, fmt.Sprintf("%d|%s|%d|%d|%d|%d|%d|%s|%s", p.ID, p.Name, p.X, p.Y, p.Food, p.Rest, p.Mood, title(p.Downed), title(p.Drafted)))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	h = New([]Plugin{plugin("p", "scripts/s.star", "def on_hour(ctx):\n    ctx.message(str(len(ctx.pawns())))\n")})
	if got := msgs(fireWith(h, scripting.OnHour, 1, nil, nil)); !reflect.DeepEqual(got, []string{"0"}) {
		t.Fatalf("nil reader: %v", got)
	}
}

func title(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func TestCtxPawnPerCall(t *testing.T) {
	t.Parallel()
	src := `def on_hour(ctx):
    for i in [9, 5, 0, -4]:
        p = ctx.pawn(i)
        ctx.message("none" if p == None else p.name)
`
	h := New([]Plugin{plugin("p", "scripts/s.star", src)})
	if got := msgs(fireWith(h, scripting.OnHour, 1, fakePawns{colonists()}, nil)); !reflect.DeepEqual(got, []string{"Bo", "none", "none", "none"}) {
		t.Fatalf("got %v", got)
	}
	h = New([]Plugin{plugin("p", "scripts/s.star", "def on_hour(ctx):\n    ctx.message(str(ctx.pawn(3)))\n")})
	if got := msgs(fireWith(h, scripting.OnHour, 1, nil, nil)); !reflect.DeepEqual(got, []string{"None"}) {
		t.Fatalf("nil reader: %v", got)
	}
	for _, bad := range []string{`ctx.pawn("x")`, `ctx.pawn()`, `ctx.pawns(1)`} {
		h = New([]Plugin{plugin("p", "scripts/s.star", "def on_hour(ctx):\n    "+bad+"\n")})
		r := fireWith(h, scripting.OnHour, 1, fakePawns{colonists()}, nil)
		if len(r.Disabled) != 1 || r.Disabled[0].Line != 2 {
			t.Errorf("%s: want a problem at line 2, got %+v", bad, r.Disabled)
		}
	}
}

func TestPawnsAreReadOnly(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"field": "ctx.pawn(3).food = 1",
		"tuple": "ctx.pawns().append(1)",
		"hash":  "d = {ctx.pawn(3): 1}",
	} {
		h := New([]Plugin{plugin("p", "scripts/s.star", "def on_hour(ctx):\n    "+body+"\n")})
		r := fireWith(h, scripting.OnHour, 1, fakePawns{colonists()}, nil)
		if len(r.Disabled) != 1 || r.Disabled[0].File != "scripts/s.star" || r.Disabled[0].Line != 2 {
			t.Errorf("%s: want a problem at scripts/s.star:2, got %+v", name, r.Disabled)
		}
	}
	v := pawnValue{colonists()[0]}
	if v.Type() != "pawn" || v.String() != "<pawn 3 Ada>" || !bool(v.Truth()) {
		t.Errorf("value basics: %s %s", v.Type(), v.String())
	}
	if n := v.AttrNames(); len(n) != 9 || n[0] != "downed" || n[8] != "y" {
		t.Errorf("attrs %v", n)
	}
	if a, err := v.Attr("nope"); a != nil || err != nil {
		t.Errorf("unknown attr: %v %v", a, err)
	}
	v.Freeze()
}

const bothHooks = `def on_pawns(ctx, ps):
    ctx.message("%s:pawns:%d" % (NAME, len(ps)))
def on_pawn(ctx, p):
    ctx.message("%s:pawn:%d" % (NAME, p.id))
`

func TestOnPawnsAndOnPawnOrder(t *testing.T) {
	t.Parallel()
	a := `NAME = "a"
` + bothHooks
	b := `NAME = "b"
` + bothHooks
	h := New([]Plugin{plugin("one", "scripts/s.star", a), plugin("two", "scripts/s.star", b)})
	due := colonists()
	pr := fakePawns{due}
	var got []string
	got = append(got, msgs(fireWith(h, scripting.OnPawns, 60, pr, due))...)
	got = append(got, msgs(fireWith(h, scripting.OnPawn, 60, pr, due))...)
	want := []string{"a:pawns:3", "b:pawns:3", "a:pawn:3", "a:pawn:9", "a:pawn:12", "b:pawn:3", "b:pawn:9", "b:pawn:12"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
	if r := fireWith(h, scripting.OnPawns, 60, pr, nil); len(r.Commands) != 0 {
		t.Fatalf("no due pawns, no call: %v", r.Commands)
	}
	if r := fireWith(h, scripting.OnPawn, 60, pr, nil); len(r.Commands) != 0 {
		t.Fatalf("no due pawns, no call: %v", r.Commands)
	}
}

func TestOnPawnSharedBudget(t *testing.T) {
	t.Parallel()
	steps := `def on_pawn(ctx, p):
    for i in range(30):
        pass
    ctx.message("ok%d" % p.id)
`
	keep := "def on_pawn(ctx, p):\n    ctx.message(\"keep\")\n"
	due := colonists()
	// One call fits the cap; the bucket's calls together do not.
	h := newHost([]Plugin{plugin("a", "scripts/s.star", steps), plugin("b", "scripts/s.star", keep)}, 500)
	one := fireWith(h, scripting.OnPawn, 60, fakePawns{due}, due[:1])
	if len(one.Disabled) != 0 || len(one.Commands) != 2 {
		t.Fatalf("one pawn fits the cap: %+v", one)
	}
	r := fireWith(h, scripting.OnPawn, 120, fakePawns{due}, due)
	if len(r.Disabled) != 1 || r.Disabled[0].Plugin != "a" {
		t.Fatalf("want a disabled by the shared steps, got %+v", r.Disabled)
	}
	for _, m := range msgs(r) {
		if strings.HasPrefix(m, "ok") {
			t.Errorf("the overrun module's commands must be dropped: %v", msgs(r))
		}
	}
	if got := msgs(r); len(got) != 3 {
		t.Errorf("other module keeps its commands: %v", got)
	}
	if h.Uses(scripting.OnPawn) != true {
		t.Error("b still defines on_pawn")
	}

	// MaxCommands+1 commands spread over pawns is an overrun.
	many := make([]scripting.Pawn, scripting.MaxCommands+1)
	for i := range many {
		many[i] = scripting.Pawn{ID: uint64(i + 1)}
	}
	h = New([]Plugin{plugin("p", "scripts/s.star", keep)})
	r = fireWith(h, scripting.OnPawn, 60, fakePawns{many}, many)
	if len(r.Disabled) != 1 || len(r.Commands) != 0 || !strings.Contains(r.Disabled[0].Msg, "on_pawn") {
		t.Fatalf("want an on_pawn budget overrun, got %+v", r)
	}
	h = New([]Plugin{plugin("p", "scripts/s.star", keep)})
	r = fireWith(h, scripting.OnPawn, 60, fakePawns{many}, many[:scripting.MaxCommands])
	if len(r.Disabled) != 0 || len(r.Commands) != scripting.MaxCommands {
		t.Fatalf("at the limit is fine: %+v", r)
	}
}

func TestOnPawnRandPerPawn(t *testing.T) {
	t.Parallel()
	src := "def on_pawn(ctx, p):\n    ctx.message(\"%d:%d\" % (p.id, ctx.rand(1000000)))\n"
	mk := func() *Host { return New([]Plugin{plugin("p", "scripts/s.star", src)}) }
	due := colonists()
	run := func(h *Host, tick uint64) []command.Command {
		return fireWith(h, scripting.OnPawn, tick, fakePawns{due}, due).Commands
	}
	a, b := run(mk(), 60), run(mk(), 60)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same events, different commands: %v %v", a, b)
	}
	num := func(c command.Command) string { return c.Name[strings.Index(c.Name, ":")+1:] }
	if num(a[0]) == num(a[1]) || num(a[1]) == num(a[2]) {
		t.Errorf("pawns share a stream: %v", a)
	}
	c := run(mk(), 120)
	if num(c[0]) == num(a[0]) && num(c[1]) == num(a[1]) {
		t.Errorf("a later tick repeats the draws: %v %v", a, c)
	}
}

func TestUsesTracksLiveModules(t *testing.T) { // not parallel: AllocsPerRun
	h := New([]Plugin{plugin("p", "scripts/s.star", "def on_hour(ctx):\n    pass\n"),
		plugin("q", "scripts/s.star", "def on_pawn(ctx, p):\n    ctx.nope()\n")})
	if h.Uses(scripting.OnPawns) || !h.Uses(scripting.OnPawn) || !h.Uses(scripting.OnHour) {
		t.Fatal("Uses must follow the defined hooks")
	}
	if n := allocs.Fewest(20, func() { h.Uses(scripting.OnPawn) }); n != 0 {
		t.Errorf("Uses allocates %v", n)
	}
	due := colonists()
	if r := fireWith(h, scripting.OnPawn, 60, fakePawns{due}, due); len(r.Disabled) != 1 {
		t.Fatalf("q should be disabled: %+v", r)
	}
	if h.Uses(scripting.OnPawn) {
		t.Error("Uses must be false once the definer is disabled")
	}
	// A module disabled in on_pawns gets no on_pawn the same tick.
	h = New([]Plugin{plugin("p", "scripts/s.star", "def on_pawns(ctx, ps):\n    fail(\"x\")\ndef on_pawn(ctx, p):\n    ctx.message(\"y\")\n")})
	pr := fakePawns{due}
	if r := fireWith(h, scripting.OnPawns, 60, pr, due); len(r.Disabled) != 1 {
		t.Fatalf("on_pawns failure: %+v", r)
	}
	if r := fireWith(h, scripting.OnPawn, 60, pr, due); len(r.Commands) != 0 || len(r.Disabled) != 0 {
		t.Fatalf("a disabled module is not called: %+v", r)
	}
}
