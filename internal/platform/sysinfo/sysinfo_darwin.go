package sysinfo

import "golang.org/x/sys/unix"

func readMemory() (total, avail uint64) {
	total, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0, 0
	}
	level, err := unix.SysctlUint32("kern.memorystatus_level")
	if err != nil {
		return total, 0
	}
	return total, availableFromLevel(total, level)
}
