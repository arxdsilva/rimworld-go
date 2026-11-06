package worldview

import (
	"strconv"
	"strings"

	"rimworld-go/internal/adapters/inbound/ebiten/lang"
	"rimworld-go/internal/application/game"
	"rimworld-go/internal/application/game/axview"
)

// Kind is a world object's kind. The order is the pick order on a shared
// tile: our maps first, then caravans, settlements, mineral sites, pods.
type Kind uint8

const (
	KindMap Kind = iota
	KindCaravan
	KindSettlement
	KindSite // a mineral site a scanner found (not yet a map)
	KindPod  // a transport pod group in flight
)

// String is the kind's word in selection keys and widget ids.
func (k Kind) String() string {
	switch k {
	case KindMap:
		return "map"
	case KindCaravan:
		return "caravan"
	case KindSettlement:
		return "settlement"
	case KindSite:
		return "site"
	}
	return "pod"
}

// Object is one thing on the world map. Key is its selection key
// ("map:2", "caravan:7", "settlement:<tile>", "site:<tile>", "pod:<n>",
// ports.OpenWorld's Select); ID its widget id ("world.obj.map.2").
type Object struct {
	Kind  Kind
	Key   string
	ID    string
	Tile  int
	Label string

	Map        game.MapView       // KindMap
	Caravan    game.CaravanView   // KindCaravan
	Settlement game.WorldSiteView // KindSettlement
	Site       axview.Site        // KindSite
	Pod        axview.PodFlight   // KindPod (Tile is its target)

	flight int  // KindPod: the whole flight in ticks (its hop count x the pace)
	razed  bool // KindMap: a settlement stood here and is gone (syncLive)
}

// Destroyed reports a map object is a site where a settlement was razed,
// or that nobody is on any more: it gets the destroyed marker while the map
// stays open.
func (o *Object) Destroyed() bool {
	return o.Kind == KindMap && o.Map.Kind == "site" && (o.razed || o.Map.Colonists == 0)
}

// newObject fills the key, id and label of an object of kind k with id n.
func newObject(k Kind, n uint64, tile int, label string) Object {
	s := strconv.FormatUint(n, 10)
	return Object{Kind: k, Key: k.String() + ":" + s, ID: "world.obj." + k.String() + "." + s, Tile: tile, Label: label}
}

// readModels are the read models the objects come from, as read once.
type readModels struct {
	maps  []game.MapView
	world game.WorldMapView
	sites []game.WorldSiteView
	ax    axview.World
	tiles int // the planet's tile count (objects off it are dropped)
}

// buildObjects lists every world object of r, in pick order (maps,
// caravans, settlements, mineral sites, pods), each kind in read order.
func buildObjects(dst []Object, r readModels) []Object { return buildObjectsIn(lang.English{}, dst, r) }

// buildObjectsIn is buildObjects in the player's language.
func buildObjectsIn(tr lang.Tr, dst []Object, r readModels) []Object {
	dst = dst[:0]
	on := func(t int) bool { return t >= 0 && t < r.tiles }
	for _, m := range r.maps {
		if on(m.Tile) {
			o := newObject(KindMap, uint64(m.ID), m.Tile, m.Label)
			o.Map = m
			dst = append(dst, o)
		}
	}
	if len(r.maps) == 0 && on(r.world.Tile) { // a game with no map list: the colony tile
		o := newObject(KindMap, 0, r.world.Tile, tr.T("Colony"))
		o.Map = game.MapView{Label: tr.T("Colony"), Kind: "colony", Tile: r.world.Tile}
		dst = append(dst, o)
	}
	for _, c := range r.world.Caravans {
		if on(c.Tile) {
			o := newObject(KindCaravan, c.ID, c.Tile, CaravanTitle(c))
			o.Caravan = c
			dst = append(dst, o)
		}
	}
	for _, s := range r.sites {
		if on(s.Tile) {
			o := newObject(KindSettlement, uint64(s.Tile), s.Tile, s.Name)
			o.Settlement = s
			dst = append(dst, o)
		}
	}
	for _, s := range r.ax.Sites {
		if on(int(s.Tile)) {
			o := newObject(KindSite, uint64(s.Tile), int(s.Tile), capFirst(s.Mineral)+" site")
			o.Site = s
			dst = append(dst, o)
		}
	}
	for i, f := range r.ax.Pods {
		if on(int(f.From)) && on(int(f.To)) {
			o := newObject(KindPod, uint64(i), int(f.To), strconv.Itoa(f.Count)+" transport pods")
			o.Pod = f
			dst = append(dst, o)
		}
	}
	return dst
}

// find is the index of the object with key k, -1 none.
func find(objs []Object, k string) int {
	for i := range objs {
		if objs[i].Key == k {
			return i
		}
	}
	return -1
}

// onTile are the indexes of the objects on tile t, in pick order (pods
// are picked where they fly, not by their target).
func onTile(dst []int, objs []Object, t int) []int {
	dst = dst[:0]
	for i := range objs {
		if objs[i].Tile == t && objs[i].Kind != KindPod {
			dst = append(dst, i)
		}
	}
	return dst
}

// CaravanTitle is a caravan's heading ("Caravan: Ada, Bo").
func CaravanTitle(c game.CaravanView) string {
	if len(c.Members) == 0 {
		return "Caravan"
	}
	return "Caravan: " + strings.Join(c.Members, ", ")
}

func capFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
