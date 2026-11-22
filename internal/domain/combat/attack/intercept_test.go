// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package attack

import "testing"

func TestInterceptScale(t *testing.T) {
	d := []int{-1, 0, 3, 5, 6, 7, 8, 11, 12, 30}
	want := []Permille{0, 0, 0, 0, 142, 285, 428, 857, 1000, 1000}
	for i, dist := range d {
		if got := InterceptScale(dist); got != want[i] {
			t.Errorf("InterceptScale(%d) = %d, want %d", dist, got, want[i])
		}
	}
}
