// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package travel

import (
	"fmt"
	"strings"
	"testing"

	"rimworld-go/internal/domain/animal"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/economy/trade"
	"rimworld-go/internal/domain/faction"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// bookHost is a travel Host that only knows the item book and records
// the stacks dropped (the rest of the interface is never called).
type bookHost struct {
	Host
	book    item.Book
	dropped []item.Stack
}

func (h *bookHost) Items() item.Book                  { return h.book }
func (h *bookHost) DropStack(st item.Stack, _, _ int) { h.dropped = append(h.dropped, st) }
func (h *bookHost) Changed()                          {}

// plugBook is core's kinds plus one plugin good of the given mass, value
// and stack limit; the second result is its kind.
func plugBook(mass, value, stack int) (item.Book, item.Kind) {
	core := item.Core()
	defs, keys := make([]item.Def, 0, item.CoreKinds+1), core.Keys()
	for k := range item.CoreKinds {
		defs = append(defs, core.Def(item.Kind(k)))
	}
	defs = append(defs, item.Def{Label: "bell", Path: "Manufactured/Components", Mass: mass, Value: value, Stack: stack})
	return item.NewBook(defs, append(keys, "frontier.bell")), item.Kind(item.CoreKinds)
}

// Ten plugin units of 700 g add 7 kg to a hold (and a pod's load), and
// the caravan's pace falls with their weight.
func TestPluginCaravanMass(t *testing.T) {
	b, bell := plugBook(700, 40, 10)
	h := Hold{Items: []item.Stack{{Kind: bell, Qty: 10}}}
	if kg := h.massKg(b, animalKg); kg != 7 {
		t.Fatalf("hold mass %d kg, want 7", kg)
	}
	if v := h.value(b); v != 400 {
		t.Fatalf("hold value %d, want 400", v)
	}
	s := &State{}
	o := Ops{core{s, &bookHost{book: b}}}
	empty := o.Pace(1, 1)
	s.cargoOf(1).addItem(item.Stack{Kind: bell, Qty: 10})
	if got := o.Pace(1, 1); got >= empty || got < 250 {
		t.Fatalf("pace %d with plugin cargo, want below the empty %d", got, empty)
	}
	core := Ops{core{&State{}, &bookHost{book: item.Core()}}}
	core.s.cargoOf(1).addItem(item.Stack{Kind: item.Steel, Qty: 10})
	if core.Pace(1, 1) < 250 {
		t.Fatal("core pace below the floor")
	}
}

// A plugin pod load lands in stacks of the def's own stack limit.
func TestPluginPodLoadSplitsByStack(t *testing.T) {
	b, bell := plugBook(700, 40, 10)
	host := &bookHost{book: b}
	o := core{&State{}, host}
	o.dropHold(&Hold{Pawns: []entity.EntityID{}, Items: []item.Stack{{Kind: bell, Qty: 25}}}, 3, 4)
	if len(host.dropped) != 3 || host.dropped[0].Qty != 10 || host.dropped[1].Qty != 10 || host.dropped[2].Qty != 5 {
		t.Fatalf("dropped %+v, want 10+10+5", host.dropped)
	}
}

// giftHost adds a faction settlement, the colony tile and a letter log to
// bookHost.
type giftHost struct {
	bookHost
	fac     faction.Faction
	letters []string
}

func (h *giftHost) ColonyTile() int                        { return 1 }
func (h *giftHost) Home() (int, int)                       { return 0, 0 }
func (h *giftHost) Faction(int) (*faction.Faction, string) { return &h.fac, "Ridge clan" }
func (h *giftHost) Letter(title string, _, _ int, format string, args ...any) {
	h.letters = append(h.letters, title+": "+fmt.Sprintf(format, args...))
}

// A gift pod landing at a settlement is worth its goods by the book: a
// plugin good's value earns goodwill and is named in the letter.
func TestPluginGiftPodValueByBook(t *testing.T) {
	b, bell := plugBook(700, 4000, 10)
	h := &giftHost{bookHost: bookHost{book: b}, fac: faction.Faction{Type: faction.Settlers, Goodwill: 0}}
	o := pods{core{&State{}, h}}
	o.land(Flight{To: 2, Mode: command.PodArriveGift, Pods: 1,
		Load: Hold{Items: []item.Stack{{Kind: bell, Qty: 5}}}})
	if len(h.letters) != 1 || !strings.Contains(h.letters[0], "worth 12240 silver") {
		t.Fatalf("letters %q, want 61.2%% of the book's 20000 silver", h.letters)
	}
	if h.fac.Goodwill <= 0 {
		t.Fatalf("goodwill %d, want a gain from the plugin goods' value", h.fac.Goodwill)
	}
}

// A group's load is its pods' holds by the book's masses.
func TestPluginPodLoadKg(t *testing.T) {
	b, bell := plugBook(700, 40, 10)
	s := &State{}
	s.holdOf(5).addItem(item.Stack{Kind: bell, Qty: 10})
	s.holdOf(6).addItem(item.Stack{Kind: bell, Qty: 20})
	o := pods{core{s, &bookHost{book: b}}}
	if kg := o.loadKg(&group{pods: []entity.EntityID{5, 6, 7}}); kg != 7+14 {
		t.Fatalf("load %d kg, want 21", kg)
	}
}

func TestMarketCapacityUsesActiveBookAndNativeCarriers(t *testing.T) {
	book, bell := plugBook(10000, 40, 10)
	o := Ops{core{&State{}, &bookHost{book: book}}}
	h := o.s.cargoOf(1)
	h.Items = []item.Stack{{Kind: bell, Qty: 3}}
	buy := trade.Result{Buy: []trade.Line{{Kind: bell, Qty: 1}}}
	incoming := []item.Stack{{Kind: bell, Qty: 1}}
	if _, why := o.PlanTrade(1, 1, buy, incoming); why == "" {
		t.Fatal("accepted 40 kg with one person")
	}
	beast := Beast{ID: 2, State: animal.State{Species: animal.Woolhorn, Tame: true}}
	if kg := caravan.PackKg(animal.BodySize(beast.State.Species)); kg < 5 {
		t.Fatalf("fixture carrier %d kg", kg)
	}
	h.Animals = []Beast{beast}
	if _, why := o.PlanTrade(1, 1, buy, incoming); why != "" {
		t.Fatal(why)
	}
	actual := carrierCapacity(1, h, TradeCarriers{Stashed: []Beast{beast}})
	if actual != int64(35+caravan.PackKg(animal.BodySize(beast.State.Species)))*1000 {
		t.Fatalf("counted duplicate: %d", actual)
	}
	beast.State.Tame = false
	if _, why := o.PlanTrade(1, 1, buy, incoming, TradeCarriers{Stashed: []Beast{beast}}); why == "" {
		t.Fatal("counted ineligible current owner")
	}
}
