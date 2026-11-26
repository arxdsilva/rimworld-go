// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package mood

import (
	"fmt"
	"testing"
)

var benchThoughts []Thought

func BenchmarkAppendActiveAtRepeatedKind(b *testing.B) {
	for _, n := range []int{4, 16, 64, 256} {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			memories := make(Memories, n)
			for i := range memories {
				memories[i] = Memory{Kind: KindWords, Subject: uint64(i + 1), Expires: 2000}
			}
			dst := make([]Thought, 0, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchThoughts = AppendActiveAt(dst[:0], memories, 1000)
			}
		})
	}
}

var benchMemories Memories

func BenchmarkMemoryGroupReplacement(b *testing.B) {
	for _, n := range []int{4, 16, 64} {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			source := make(Memories, n)
			for i := range source {
				kind := KindWords
				if i%4 == 0 {
					kind = SleptOutside
				}
				source[i] = Memory{Kind: kind, Subject: uint64(i + 1), Expires: 2000, Label: "some memory"}
			}
			work := make(Memories, n, n+1)
			b.Run("Add", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					copy(work, source)
					benchMemories = work[:n].Add(SleptInBed, 1000)
				}
			})
			b.Run("AddFor", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					copy(work, source)
					benchMemories = work[:n].AddFor(SleptInBed, 99, 1000)
				}
			})
			b.Run("Forget", func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					copy(work, source)
					benchMemories = work[:n].ForgetGroupOf(SleptOutside)
				}
			})
		})
	}
}

func BenchmarkAppendActiveAtNoDecay(b *testing.B) {
	for _, n := range []int{4, 16, 64, 256} {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			memories := make(Memories, n)
			for i := range memories {
				memories[i] = Memory{Kind: Grief, Value: -20, Subject: uint64(i + 1), Expires: 2000}
			}
			dst := make([]Thought, 0, n)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchThoughts = AppendActiveAt(dst[:0], memories, 1000)
			}
		})
	}
}

func BenchmarkMemoryInsertWithExpired(b *testing.B) {
	for _, n := range []int{4, 64, 256} {
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			src := make(Memories, n)
			for i := range src {
				src[i] = Memory{Kind: KindWords, Subject: uint64(i + 1), Expires: 900}
			}
			work := make(Memories, n, n+1)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				copy(work, src)
				benchMemories = work[:n].AddFor(Insulted, 9, 1000)
			}
		})
	}
}
