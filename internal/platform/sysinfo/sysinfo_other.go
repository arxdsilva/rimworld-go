//go:build !darwin && !linux && !windows

package sysinfo

func readMemory() (total, avail uint64) { return 0, 0 }
