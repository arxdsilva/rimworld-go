// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package filesave

import (
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

// A queued delete is a completion barrier, never a coalescible save.
func TestQueueSaveAfterPendingDelete(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	q := NewQueue(s)
	defer q.Close()
	started, gate := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	defer release()
	first := true
	q.saveTo = func(path string, data []byte) error {
		if first {
			first = false
			started <- struct{}{}
			<-gate
		}
		return writeSave(path, data)
	}
	if err := q.put("a", raw("old")); err != nil {
		t.Fatal(err)
	}
	<-started
	done := make(chan error, 1)
	go func() { done <- q.Delete("a") }()
	deadline := time.Now().Add(5 * time.Second)
	for !deleteQueued(q) {
		if time.Now().After(deadline) {
			t.Fatal("delete never queued")
		}
		runtime.Gosched()
	}
	q.mu.Lock()
	deleted := q.jobs[0].deleted
	q.mu.Unlock()
	if err := q.put("a", raw("new")); err != nil {
		t.Fatal(err)
	}
	q.mu.Lock()
	kept := len(q.jobs) == 2 && q.jobs[0].deleted == deleted && q.jobs[1].deleted == nil
	q.mu.Unlock()
	if !kept {
		// Rescue the orphaned waiter so the regression itself does not leak.
		deleted <- errors.New("delete completion was overwritten")
		<-done
		t.Fatal("new save replaced an outstanding Delete job; Delete cannot return")
	}
	release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	q.Wait()
	if !exists(t, s.Dir+"/a"+Ext) {
		t.Fatal("save after delete did not recreate slot")
	}
}

func TestQueueRepeatedDeleteCompletesBoth(t *testing.T) {
	q := NewQueue(Store{Dir: t.TempDir()})
	defer q.Close()
	started, gate := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	defer release()
	q.saveTo = func(path string, data []byte) error { started <- struct{}{}; <-gate; return writeSave(path, data) }
	if err := q.put("a", raw("old")); err != nil {
		t.Fatal(err)
	}
	<-started
	firstDone, secondDone := make(chan error, 1), make(chan error, 1)
	go func() { firstDone <- q.Delete("a") }()
	deadline := time.Now().Add(5 * time.Second)
	for !deleteQueued(q) {
		if time.Now().After(deadline) {
			t.Fatal("first delete never queued")
		}
		runtime.Gosched()
	}
	q.mu.Lock()
	first := q.jobs[0].deleted
	q.mu.Unlock()
	go func() { secondDone <- q.Delete("a") }()
	for {
		q.mu.Lock()
		n := len(q.jobs)
		changed := n > 0 && q.jobs[0].deleted != first
		q.mu.Unlock()
		if n == 2 || changed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("second delete never queued")
		}
		runtime.Gosched()
	}
	q.mu.Lock()
	found := false
	for _, j := range q.jobs {
		found = found || j.deleted == first
	}
	q.mu.Unlock()
	if !found {
		first <- errors.New("delete completion was discarded")
		<-firstDone
		t.Fatal("second Delete discarded the first completion barrier")
	}
	release()
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
}
