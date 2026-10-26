package sysinfo

import (
	"strings"
	"testing"
)

func TestParseMeminfo(t *testing.T) {
	const sample = `MemTotal:       16303192 kB
MemFree:          812344 kB
MemAvailable:    9234108 kB
Buffers:          123456 kB
`
	total, avail := parseMeminfo(strings.NewReader(sample))
	if total != 16303192*1024 || avail != 9234108*1024 {
		t.Fatalf("total %d avail %d", total, avail)
	}
	// an old kernel without MemAvailable: free stands in
	total, avail = parseMeminfo(strings.NewReader("MemTotal: 2048 kB\nMemFree: 512 kB\n"))
	if total != 2048*1024 || avail != 512*1024 {
		t.Fatalf("fallback total %d avail %d", total, avail)
	}
	if total, avail = parseMeminfo(strings.NewReader("garbage\n")); total != 0 || avail != 0 {
		t.Fatalf("garbage gave %d %d", total, avail)
	}
}

func TestAvailableFromLevel(t *testing.T) {
	if got := availableFromLevel(16<<30, 63); got != 16<<30*63/100 {
		t.Fatalf("63%% of 16 GiB = %d", got)
	}
	if got := availableFromLevel(16<<30, 0); got != 0 {
		t.Fatalf("unknown level gave %d", got)
	}
	if got := availableFromLevel(16<<30, 140); got != 16<<30 {
		t.Fatalf("a level over 100 must clamp, got %d", got)
	}
}

func TestReadOnThisMachine(t *testing.T) {
	i := Read()
	if i.Cores < 1 {
		t.Fatalf("cores %d", i.Cores)
	}
	if i.Total != 0 && i.Available > i.Total {
		t.Fatalf("available %d above total %d", i.Available, i.Total)
	}
	if i.TotalGiB() != float64(i.Total)/GiB {
		t.Fatal("TotalGiB")
	}
}
