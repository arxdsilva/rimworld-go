// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package difficulty

import "testing"

var benchAdapt Adapter

func BenchmarkAdaptationDay(b *testing.B) {
	s := Of(Seasoned)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchAdapt.HalfDay(s, 100)
	}
}
