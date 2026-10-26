package sysinfo

import "os"

func readMemory() (total, avail uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	return parseMeminfo(f)
}
