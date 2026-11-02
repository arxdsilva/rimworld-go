package travel

import (
	"fmt"
	"slices"

	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/clock"
	"rimworld-go/internal/domain/sim/command"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// Launching and landing (wiki: Transport pod, Pod launcher): a group
// fires to any world tile its fuel reaches (2.25 chemfuel a tile a pod,
// 66 tiles at most); the pods are spent. They land on the colony (at its
// drop spot), as a gift to a settlement (goodwill by the goods' market
// value, wiki), or set their passengers down as a caravan: visiting a
// friendly settlement, or waiting on any other tile (a mineral site is
// mined). Founding a new colony and attacking a settlement need a second
// map, which the game does not have.

// mineTicks is how long a caravan works a mineral site (original tuning:
// the wiki gives ten days to mine a site's map, which the game has not).
const mineTicks = 2 * clock.TicksPerDay

// LaunchReason is why pod group of pod would not fly to tile dest in mode
// ("" it would), with the distance in tiles.
func (o pods) LaunchReason(pod entity.EntityID, dest int, mode uint8) (string, int) {
	p, home := o.h.Planet(), o.h.ColonyTile()
	if p == nil || home < 0 {
		return "No world map: the pods have nowhere to fly.", 0
	}
	gr, ok := o.groupOf(pod)
	if !ok {
		return "No transport pod.", 0
	}
	if dest < 0 || dest >= p.TileCount() {
		return "Pick a tile on the world map.", 0
	}
	if gr.unfed() > 0 {
		return "Every pod needs a pod launcher beside it.", 0
	}
	load := o.groupHold(gr)
	if load.empty() {
		return "The pods are empty: load them first.", 0
	}
	dist := p.Hops(home, dest, caravan.PodMaxRange+1)
	if r, _ := o.rangeOf(gr); dist > r {
		return fmt.Sprintf("Out of range: %d tiles away, the fuel reaches %d.", dist, r), dist
	}
	_, settled, hostile, human := o.settlement(dest)
	switch {
	case mode == command.PodArriveGift && !settled:
		return "A gift needs a settlement to land on.", dist
	case mode == command.PodArriveGift && len(load.Pawns) > 0:
		return "People cannot be sent as a gift.", dist
	case dest == home || mode == command.PodArriveGift:
	case settled && (hostile || !human):
		return "They would shoot the pods down: attacking a settlement is not in this game.", dist
	case len(load.Pawns) == 0:
		return "Nobody aboard to land there: the goods would be lost.", dist
	}
	return "", dist
}

// groupHold is everything the group's pods hold, merged (a copy).
func (o pods) groupHold(gr *group) Hold {
	var h Hold
	for _, p := range gr.pods {
		if ph := o.s.holds[p]; ph != nil {
			h.merge(ph)
		}
	}
	return h.clone()
}

// cmdLaunch fires pod Target's group to tile Count, landing per Mode.
func (o pods) cmdLaunch(c command.Command) {
	pod, dest := entity.EntityID(c.Target), c.Count
	gr, ok := o.groupOf(pod)
	if !ok {
		return
	}
	b, _ := o.h.Building(gr.leader)
	why, dist := o.LaunchReason(pod, dest, c.Mode)
	if why != "" {
		o.h.Message(b.X, b.Y, "Can't launch: "+why)
		return
	}
	for _, l := range gr.launchers {
		o.h.SetFuel(l, max(0, o.h.Fuel(l)-caravan.PodFuel(dist, gr.fedBy(l))))
	}
	load := o.groupHold(gr)
	pods := slices.Clone(gr.pods)
	for _, p := range pods {
		delete(o.s.holds, p)
	}
	delete(o.s.wants, gr.leader)
	for _, p := range pods {
		o.h.Remove(p)
	}
	now := o.h.Now()
	o.s.nextID++
	f := Flight{ID: o.s.nextID, From: o.h.ColonyTile(), To: dest, Depart: now,
		Arrive: now + uint64(max(1, dist)*caravan.PodTicksPerTile), Mode: c.Mode, Pods: len(pods), Load: load}
	o.s.flights = append(o.s.flights, f)
	o.h.Letter("Transport pods launched", fmt.Sprintf("%d transport pods left for %s; they land in about %d hours.",
		len(pods), o.tileName(dest), max(1, int(f.Arrive-now)/clock.TicksPerHour)), b.X, b.Y)
	o.h.Changed()
}

// tileName names world tile t: home, its settlement or its number.
func (o pods) tileName(t int) string {
	if t == o.h.ColonyTile() {
		return "the colony"
	}
	if n, ok, _, _ := o.settlement(t); ok {
		return n
	}
	if s, ok := scanner(o).siteAt(t); ok {
		return "the " + s.Mineral.String() + " site"
	}
	return fmt.Sprintf("tile %d", t)
}

// flightTick lands the flights that arrive by now, in launch order.
func (o pods) flightTick(now uint64) {
	for i := 0; i < len(o.s.flights); i++ {
		if f := o.s.flights[i]; now >= f.Arrive {
			o.s.flights = slices.Delete(o.s.flights, i, i+1)
			i--
			o.land(f)
		}
	}
}

// land sets flight f down.
func (o pods) land(f Flight) {
	hx, hy := o.h.Home()
	load := f.Load
	if f.To == o.h.ColonyTile() {
		o.dropHold(&load, hx, hy)
		o.h.Letter("Transport pods landed", fmt.Sprintf("%d transport pods came down at the colony.", f.Pods), hx, hy)
		return
	}
	name, settled, _, _ := o.settlement(f.To)
	if f.Mode == command.PodArriveGift && settled {
		gw := o.gift(f.To, load.value())
		o.h.Letter("Gift delivered", fmt.Sprintf("%s received the pods' goods, worth %d silver (%+d goodwill).", name, load.value(), gw), hx, hy)
		return
	}
	if len(load.Pawns) == 0 {
		o.h.Letter("Transport pods lost", "The pods came down with nobody aboard; their goods are lost.", hx, hy)
		return
	}
	var members, animals []uint64
	for _, p := range load.Pawns {
		members = append(members, uint64(p))
	}
	for _, b := range load.Animals {
		animals = append(animals, uint64(b.ID))
	}
	cid := o.h.Land(members, animals, f.To)
	if len(load.Animals) > 0 || len(load.Items) > 0 {
		h := o.s.cargoOf(cid)
		h.Animals = append(h.Animals, load.Animals...)
		for _, st := range load.Items {
			h.addItem(st)
		}
	}
}

// Mine is a caravan's arrival on tile t at now: on a mineral site it
// takes the lump as cargo and works there until the returned tick (ok
// false: no site there).
func (o pods) Mine(cid uint64, t int, now uint64) (uint64, bool) {
	i := slices.IndexFunc(o.s.sites, func(s Site) bool { return s.Tile == t })
	if i < 0 {
		return 0, false
	}
	s := o.s.sites[i]
	o.s.sites = slices.Delete(o.s.sites, i, i+1)
	o.s.cargoOf(cid).addItem(item.Stack{Kind: s.Mineral, Qty: s.Qty})
	hx, hy := o.h.Home()
	o.h.Letter("Mining the site", fmt.Sprintf("The caravan reached the %s site and mines it: %d %s, ready to bring home in two days.",
		s.Mineral, s.Qty, s.Mineral), hx, hy)
	o.h.Changed()
	return now + mineTicks, true
}

// Home sets caravan cid's cargo down around (x, y), where it came back
// onto the map.
func (o pods) Home(cid uint64, x, y int) {
	h := o.s.cargo[cid]
	if h == nil {
		return
	}
	delete(o.s.cargo, cid)
	o.dropHold(h, x, y)
}

// cargoTick forgets the cargo of caravans lost on the road.
func (o pods) cargoTick() {
	for _, cid := range sortedKeys(o.s.cargo) {
		if !o.h.Caravan(cid) {
			delete(o.s.cargo, cid)
		}
	}
}
