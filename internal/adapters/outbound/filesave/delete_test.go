package filesave

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

func TestStoreDeleteRemovesSidecar(t *testing.T) {
	t.Parallel()
	s := Store{Dir: t.TempDir()}
	for _, f := range []string{"My-colony" + Ext, "My-colony" + ViewExt, "Other" + Ext, "Lone" + Ext} {
		if err := os.WriteFile(filepath.Join(s.Dir, f), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Delete("My colony"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("Lone"); err != nil {
		t.Fatalf("a save without a sidecar: %v", err)
	}
	if err := s.Delete("My colony"); err != nil {
		t.Fatalf("deleting a deleted save: %v", err)
	}
	ents, _ := os.ReadDir(s.Dir)
	if len(ents) != 1 || ents[0].Name() != "Other"+Ext {
		t.Fatalf("left %v, want only Other", ents)
	}
}

// The delete waits for the write in flight (here gated), drops the slot's
// waiting save and view, and leaves the other slots' jobs queued.
func TestQueueDeleteWaitsForInFlightWrite(t *testing.T) {
	t.Parallel()
	s := Store{Dir: t.TempDir()}
	q := NewQueue(s)
	defer q.Close()
	started, gate := make(chan struct{}, 8), make(chan struct{})
	q.saveTo = func(path string, raw []byte) error { started <- struct{}{}; <-gate; return writeSave(path, raw) }
	if err := q.put("a", raw("first")); err != nil {
		t.Fatal(err)
	}
	<-started // "a" is being written
	for _, err := range []error{q.put("a", raw("second")), q.SaveView("a", ViewFile{CX: 1, CY: 1, Zoom: 1}), q.put("b", raw("b"))} {
		if err != nil {
			t.Fatal(err)
		}
	}
	done := make(chan error, 1)
	go func() { done <- q.Delete("a") }()
	select {
	case err := <-done:
		t.Fatalf("Delete returned (%v) while a write was in flight", err)
	default:
	}
	close(gate)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	q.Wait()
	if exists(t, filepath.Join(s.Dir, "a"+Ext)) || exists(t, filepath.Join(s.Dir, "a"+ViewExt)) {
		t.Fatal("the in-flight write outlived the delete")
	}
	if !exists(t, filepath.Join(s.Dir, "b"+Ext)) {
		t.Fatal("another slot's save was dropped")
	}
	var got []string
	for _, r := range q.Done() {
		got = append(got, r.Slot)
	}
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("results %v, want the in-flight a then b (a's waiting save and view dropped)", got)
	}
}

// A slot whose save failed is forgotten on delete: a save queued after it
// writes the slot again, and its view is no longer skipped.
func TestQueueSaveAfterDeleteRecreates(t *testing.T) {
	t.Parallel()
	s := Store{Dir: t.TempDir()}
	q := NewQueue(s)
	defer q.Close()
	fail := true
	q.saveTo = func(path string, raw []byte) error {
		if fail {
			return errors.New("disk full")
		}
		return writeSave(path, raw)
	}
	if err := q.put("a", raw("x")); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	if rs := q.Done(); len(rs) != 1 || rs[0].Err == nil {
		t.Fatalf("first save %+v, want a failure", rs)
	}
	if err := q.Delete("a"); err != nil {
		t.Fatal(err)
	}
	fail = false // the worker is idle: Wait and Delete returned
	if err := q.put("a", raw("y")); err != nil {
		t.Fatal(err)
	}
	if err := q.SaveView("a", ViewFile{CX: 2, CY: 3, Zoom: 1}); err != nil {
		t.Fatal(err)
	}
	q.Wait()
	if !exists(t, filepath.Join(s.Dir, "a"+Ext)) || !exists(t, filepath.Join(s.Dir, "a"+ViewExt)) {
		t.Fatal("the save after the delete did not write the slot and its view")
	}
	if err := q.Delete("a"); err != nil || exists(t, filepath.Join(s.Dir, "a"+Ext)) {
		t.Fatalf("second delete: %v", err)
	}
	q.Close()
	if err := q.Delete("a"); !errors.Is(err, ErrClosed) {
		t.Fatalf("Delete after Close: %v", err)
	}
}
