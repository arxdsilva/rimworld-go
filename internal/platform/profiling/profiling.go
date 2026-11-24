// SPDX-License-Identifier: GPL-3.0-only
// Copyright (C) 2026 Arthur Rodrigues Xavier da Silva

package profiling

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
)

// Start starts the CPU profile (when cpuPath is set) and the execution trace
// (when tracePath is set); the returned func stops them. A file that can't
// be made is logged and skipped.
func Start(cpuPath, tracePath string) (stop func()) {
	var stops []func()
	if cpuPath != "" {
		if f, err := os.Create(filepath.Clean(cpuPath)); err == nil && pprof.StartCPUProfile(f) == nil {
			stops = append(stops, func() { pprof.StopCPUProfile(); closeLogged("cpuprofile", f) })
		} else {
			log.Printf("cpuprofile: %v", err)
		}
	}
	if tracePath != "" {
		if f, err := os.Create(filepath.Clean(tracePath)); err == nil && trace.Start(f) == nil {
			stops = append(stops, func() { trace.Stop(); closeLogged("trace", f) })
		} else {
			log.Printf("trace: %v", err)
		}
	}
	return func() {
		for _, s := range stops {
			s()
		}
	}
}

// closeLogged closes f and logs a failure: a profile cut short is worth knowing.
func closeLogged(what string, f *os.File) {
	if err := f.Close(); err != nil {
		log.Printf("%s: close: %v", what, err)
	}
}

// WriteHeap writes a heap profile to path after a GC; "" writes nothing.
func WriteHeap(path string) {
	if path == "" {
		return
	}
	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		log.Printf("memprofile: %v", err)
		return
	}
	defer f.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		log.Printf("memprofile: %v", err)
	}
}
