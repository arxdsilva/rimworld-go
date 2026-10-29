package care

import (
	"maps"
	"testing"

	"rimworld-go/internal/domain/sim/entity"
)

func TestCarriesPickAndDrop(t *testing.T) {
	cs := NewCarries()
	cs.Pick(5, 9)
	if cs.Patient(5) != 9 || cs.Carrier(9) != 5 || cs.Len() != 1 {
		t.Fatalf("after Pick(5,9): patient %d carrier %d len %d", cs.Patient(5), cs.Carrier(9), cs.Len())
	}
	p, ok := cs.Drop(5)
	if !ok || p != 9 {
		t.Fatalf("Drop(5) = %d, %v", p, ok)
	}
	if cs.Patient(5) != 0 || cs.Carrier(9) != 0 || cs.Len() != 0 {
		t.Fatal("Drop left the pair behind")
	}
	if _, ok := cs.Drop(5); ok {
		t.Fatal("second Drop reported a patient")
	}
}

func TestCarriesPickReplacesBothSides(t *testing.T) {
	cs := NewCarries()
	cs.Pick(1, 10)
	cs.Pick(2, 10) // 10 changes hands: 1 lets go
	if cs.Patient(1) != 0 || cs.Carrier(10) != 2 {
		t.Fatalf("patient changed hands: P(1)=%d C(10)=%d", cs.Patient(1), cs.Carrier(10))
	}
	cs.Pick(2, 11) // 2 puts 10 down for 11
	if cs.Carrier(10) != 0 || cs.Carrier(11) != 2 || cs.Len() != 1 {
		t.Fatalf("carrier switched patient: C(10)=%d C(11)=%d len %d", cs.Carrier(10), cs.Carrier(11), cs.Len())
	}
	cs.Pick(0, 3)
	cs.Pick(3, 0)
	if cs.Len() != 1 {
		t.Fatal("a zero id entered the index")
	}
}

func TestCarriesForgetBothRoles(t *testing.T) {
	cs := NewCarries()
	cs.Pick(1, 2)
	cs.Pick(3, 4)
	cs.Forget(2) // as a patient
	cs.Forget(3) // as a carrier
	if cs.Len() != 0 || cs.Patient(1) != 0 || cs.Carrier(4) != 0 {
		t.Fatalf("Forget left %d pairs", cs.Len())
	}
	cs.Forget(99) // unknown: no-op
}

func TestCarriesEachAscending(t *testing.T) {
	cs := NewCarries()
	for _, c := range []entity.EntityID{7, 3, 9, 1} {
		cs.Pick(c, c+100)
	}
	cs.Drop(9)
	var got []entity.EntityID
	cs.Each(func(c, p entity.EntityID) {
		if p != c+100 {
			t.Errorf("Each(%d) patient %d", c, p)
		}
		got = append(got, c)
	})
	want := []entity.EntityID{1, 3, 7}
	if len(got) != len(want) {
		t.Fatalf("Each order %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Each order %v, want %v", got, want)
		}
	}
}

func TestCarriesExportImport(t *testing.T) {
	cs := NewCarries()
	cs.Pick(4, 40)
	cs.Pick(2, 20)
	m := cs.Export()
	m[99] = 990 // a copy: the index is untouched
	if cs.Len() != 2 {
		t.Fatal("Export shares the index map")
	}
	delete(m, 99)
	back := NewCarries()
	back.Pick(8, 80)
	back.Import(m)
	if !maps.Equal(back.Export(), map[entity.EntityID]entity.EntityID{4: 40, 2: 20}) || back.Carrier(80) != 0 {
		t.Fatalf("Import = %v", back.Export())
	}
	if back.Carrier(20) != 2 || back.Carrier(40) != 4 {
		t.Fatal("Import did not rebuild the patient side")
	}
	back.Import(nil) // a save without the field keeps the index
	if back.Len() != 2 {
		t.Fatal("Import(nil) cleared the index")
	}
	dup := NewCarries()
	dup.Import(map[entity.EntityID]entity.EntityID{5: 50, 6: 50})
	if dup.Len() != 1 || dup.Carrier(50) != 6 {
		t.Fatalf("a patient named twice: len %d carrier %d", dup.Len(), dup.Carrier(50))
	}
}
