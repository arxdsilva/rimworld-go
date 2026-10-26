// Package sysinfo reads how much memory the machine has and how many
// cores, without cgo. The game uses it to pick graphics settings that run
// smoothly and to warn on small machines; cmd/rambudget uses it to size
// parallel agent work.
package sysinfo

import (
	"bufio"
	"io"
	"runtime"
	"strconv"
	"strings"
)

// GiB is a gibibyte in bytes.
const GiB = 1 << 30

// Info is what the machine has. A zero Total or Available means the
// platform could not tell.
type Info struct {
	Total     uint64 // physical memory, bytes
	Available uint64 // memory free for new work right now, bytes
	Cores     int
}

// Read reads this machine's memory and core count.
func Read() Info {
	total, avail := readMemory()
	if total != 0 && avail > total {
		avail = total
	}
	return Info{Total: total, Available: avail, Cores: runtime.NumCPU()}
}

// TotalGiB and AvailableGiB are the sizes in GiB.
func (i Info) TotalGiB() float64     { return float64(i.Total) / GiB }
func (i Info) AvailableGiB() float64 { return float64(i.Available) / GiB }

// parseMeminfo reads Linux /proc/meminfo: MemTotal and MemAvailable (MemFree
// on kernels without it), in bytes.
func parseMeminfo(r io.Reader) (total, avail uint64) {
	var free uint64
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 {
			continue
		}
		n, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			continue
		}
		if len(f) > 2 && f[2] == "kB" {
			n *= 1024
		}
		switch f[0] {
		case "MemTotal:":
			total = n
		case "MemAvailable:":
			avail = n
		case "MemFree:":
			free = n
		}
	}
	if avail == 0 {
		avail = free
	}
	return total, avail
}

// availableFromLevel is macOS's free share: the kernel's memory status
// level is the percentage of memory free for new work.
func availableFromLevel(total uint64, level uint32) uint64 {
	if level == 0 {
		return 0
	}
	if level > 100 {
		level = 100
	}
	return total * uint64(level) / 100
}
