package ebiten

// journey_wave_c_helpers_test.go: fixtures for the wave C journeys (PRD-2
// quality and stuff, BED-2 room stats, DEF-2 weapons, armor and
// explosions). Item kinds the catalogue adds are looked up by name so a
// missing one fails with a message naming it.

import (
	"fmt"
	"reflect"
	"strings"

	"rimworld-go/internal/application/game"
	"rimworld-go/internal/domain/economy/item"
	"rimworld-go/internal/domain/sim/command"
)

// wcKind is the item kind with that name, fatal when the catalogue lacks it.
func wcKind(j *journey, name string) item.Kind {
	j.t.Helper()
	k, ok := item.ParseKind(name)
	if !ok {
		j.t.Fatalf("item kind %q does not exist yet (DEF-2/PRD-4 catalogue)", name)
	}
	return k
}

// wcQualityWords are the quality levels an item line may name.
var wcQualityWords = []string{"awful", "poor", "normal", "good", "excellent", "masterwork", "legendary"}

// wcHasQuality reports whether text names a quality level.
func wcHasQuality(text string) (string, bool) {
	low := strings.ToLower(text)
	for _, w := range wcQualityWords {
		if strings.Contains(low, w) {
			return w, true
		}
	}
	return "", false
}

// wcWall builds a wooden wall ring around the rectangle x0..x1, y0..y1 with
// a door at (dx, dy) on its border.
func wcRoom(j *journey, x0, y0, x1, y1, dx, dy int) {
	j.t.Helper()
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if x != x0 && x != x1 && y != y0 && y != y1 {
				continue
			}
			kind := "wall"
			if x == dx && y == dy {
				kind = "door"
			}
			if !j.g.BuildNow(kind, "wood", "south", x, y) {
				j.t.Fatalf("no %s at (%d, %d)", kind, x, y)
			}
		}
	}
}

// wcRoomAt is the room view covering tile (x, y) and its id.
func wcRoomAt(j *journey, x, y int) (game.RoomView, bool) {
	j.update(1)
	return j.g.RoomInfo(x, y)
}

// wcPrioritize forces colonist pawn to equip or wear the stack at (x, y).
func wcPrioritize(j *journey, pawn uint64, x, y int) {
	j.g.Enqueue(command.Command{Kind: command.Prioritize, Pawn: pawn, Order: command.OrderEquip, X: x, Y: y})
	j.g.ApplyPending()
}

// wcField reads exported field name of a struct value by reflection, so a
// read-model field a parallel agent has not landed fails the journey with
// a message instead of breaking the package build.
func wcField(j *journey, v any, name string) reflect.Value {
	j.t.Helper()
	f := reflect.Indirect(reflect.ValueOf(v)).FieldByName(name)
	if !f.IsValid() {
		j.t.Fatalf("%T has no field %s yet", v, name)
	}
	return f
}

// wcNum is a numeric field as float64.
func wcNum(j *journey, v any, name string) float64 {
	j.t.Helper()
	f := wcField(j, v, name)
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(f.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(f.Uint())
	case reflect.Float32, reflect.Float64:
		return f.Float()
	}
	j.t.Fatalf("%T.%s is %s, not a number", v, name, f.Kind())
	return 0
}

// wcStr is a string field (or its String form).
func wcStr(j *journey, v any, name string) string {
	j.t.Helper()
	f := wcField(j, v, name)
	if f.Kind() == reflect.String {
		return f.String()
	}
	if s, ok := f.Interface().(interface{ String() string }); ok {
		return s.String()
	}
	return fmt.Sprint(f.Interface())
}
