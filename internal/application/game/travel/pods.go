package travel

import (
	"slices"

	"rimworld-go/internal/domain/building/construction"
	"rimworld-go/internal/domain/sim/entity"
	"rimworld-go/internal/domain/world/caravan"
)

// Pod groups (wiki: Transport pod, Pod launcher): transport pods that
// touch edge to edge load and launch as one group, named by its lowest
// pod id. Each pod is fed by a pod launcher beside it (sharing an edge;
// the lowest such launcher), which pays its chemfuel: 2.25 a tile a pod,
// so a launcher feeding two pods reaches half as far. The group's range
// is its shortest launcher's.

// group is one pod group: its pods (ascending), each pod's launcher (0
// none, parallel to pods) and the launchers that feed it (ascending).
type group struct {
	leader    entity.EntityID
	pods      []entity.EntityID
	feed      []entity.EntityID
	launchers []entity.EntityID
}

// groupCache is the groups of one structure version.
type groupCache struct {
	ok   bool
	ver  uint64
	list []group
	of   map[entity.EntityID]int // pod or launcher -> index in list
}

// groups is the pod groups now (rebuilt when the structure changes).
func (o pods) groups() *groupCache {
	c := &o.s.groups
	if c.ok && c.ver == o.h.StructVer() {
		return c
	}
	*c = groupCache{ok: true, ver: o.h.StructVer(), of: map[entity.EntityID]int{}}
	podAt := map[[2]int]entity.EntityID{}
	launcherAt := map[[2]int]entity.EntityID{}
	var pods []entity.EntityID
	for _, id := range o.h.BuildingIDs() {
		b, ok := o.h.Building(id)
		if !ok {
			continue
		}
		switch b.Kind {
		case construction.TransportPod:
			podAt[[2]int{b.X, b.Y}] = id
			pods = append(pods, id)
		case construction.PodLauncher:
			cells, _, _ := construction.Footprint(b.Kind, b.Facing, b.X, b.Y)
			for _, cl := range cells {
				launcherAt[cl] = id
			}
		}
	}
	slices.Sort(pods)
	seen := map[entity.EntityID]bool{}
	for _, start := range pods {
		if seen[start] {
			continue
		}
		var gr group
		queue := []entity.EntityID{start}
		seen[start] = true
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			gr.pods = append(gr.pods, id)
			b, _ := o.h.Building(id)
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if n, ok := podAt[[2]int{b.X + d[0], b.Y + d[1]}]; ok && !seen[n] {
					seen[n] = true
					queue = append(queue, n)
				}
			}
		}
		slices.Sort(gr.pods)
		gr.leader = gr.pods[0]
		for _, id := range gr.pods {
			b, _ := o.h.Building(id)
			var l entity.EntityID
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if n, ok := launcherAt[[2]int{b.X + d[0], b.Y + d[1]}]; ok && (l == 0 || n < l) {
					l = n
				}
			}
			gr.feed = append(gr.feed, l)
			if l != 0 && !slices.Contains(gr.launchers, l) {
				gr.launchers = append(gr.launchers, l)
			}
		}
		slices.Sort(gr.launchers)
		i := len(c.list)
		for _, id := range gr.pods {
			c.of[id] = i
		}
		for _, l := range gr.launchers {
			if _, taken := c.of[l]; !taken {
				c.of[l] = i
			}
		}
		c.list = append(c.list, gr)
	}
	o.rekeyWants(c)
	return c
}

// rekeyWants moves each manifest to its group's leader (a lower pod built
// later leads the group now) and drops those whose pods are all gone.
func (o pods) rekeyWants(c *groupCache) {
	for _, k := range sortedKeys(o.s.wants) {
		i, ok := c.of[k]
		if !ok || c.list[i].leader != k && !slices.Contains(c.list[i].pods, k) {
			delete(o.s.wants, k)
			continue
		}
		if l := c.list[i].leader; l != k {
			if o.s.wants[l] == nil {
				o.s.wants[l] = o.s.wants[k]
			}
			delete(o.s.wants, k)
		}
	}
}

// groupOf is the group pod (or launcher) id belongs to.
func (o pods) groupOf(id entity.EntityID) (*group, bool) {
	c := o.groups()
	i, ok := c.of[id]
	if !ok {
		return nil, false
	}
	return &c.list[i], true
}

// loadKg is what the group's pods hold now, in kilograms.
func (o pods) loadKg(gr *group) int {
	kg := 0
	for _, p := range gr.pods {
		if h := o.s.holds[p]; h != nil {
			kg += h.massKg(animalKg)
		}
	}
	return kg
}

// capKg is the group's lift (150 kg a pod).
func capKg(gr *group) int { return len(gr.pods) * caravan.PodCapacityKg }

// fedBy is how many of the group's pods launcher l feeds.
func (gr *group) fedBy(l entity.EntityID) int {
	n := 0
	for _, f := range gr.feed {
		if f == l {
			n++
		}
	}
	return n
}

// unfed is how many pods have no launcher beside them.
func (gr *group) unfed() int { return gr.fedBy(0) }

// rangeOf is how far the group flies on its launchers' fuel (0 when a pod
// has no launcher), and the least chemfuel among them.
func (o pods) rangeOf(gr *group) (tiles, fuel int) {
	if len(gr.launchers) == 0 {
		return 0, 0
	}
	tiles, fuel = caravan.PodMaxRange, 1<<30
	for _, l := range gr.launchers {
		f := o.h.Fuel(l)
		fuel = min(fuel, f)
		tiles = min(tiles, caravan.PodRange(f, gr.fedBy(l)))
	}
	if gr.unfed() > 0 {
		tiles = 0
	}
	return tiles, fuel
}

// podWithRoom is the group's first pod that takes kg more (else the one
// with the most room left).
func (o pods) podWithRoom(gr *group, kg int) entity.EntityID {
	best, room := gr.leader, -1<<30
	for _, p := range gr.pods {
		left := caravan.PodCapacityKg
		if h := o.s.holds[p]; h != nil {
			left -= h.massKg(animalKg)
		}
		if left >= kg {
			return p
		}
		if left > room {
			best, room = p, left
		}
	}
	return best
}
