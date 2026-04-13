//go:build linux

package hostmem

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Stats struct {
	HostTotalBytes     uint64
	HostAvailableBytes uint64
	ProcessRSSBytes    uint64
}

func ReadStats() (Stats, error) {
	var s Stats
	mt, ma, err := meminfoTotalAvailable()
	if err != nil {
		return s, err
	}
	s.HostTotalBytes = mt
	s.HostAvailableBytes = ma
	rss, err := selfVmRSSKBytes()
	if err != nil {
		return s, err
	}
	s.ProcessRSSBytes = rss * 1024
	return s, nil
}

func meminfoTotalAvailable() (totalBytes, availableBytes uint64, err error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	var (
		memTotal, memFree, memAvailable uint64
		buffers, cached, sReclaimable   uint64
	)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			memTotal, err = parseMeminfoKBLine(line)
		case strings.HasPrefix(line, "MemFree:"):
			memFree, err = parseMeminfoKBLine(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			memAvailable, err = parseMeminfoKBLine(line)
		case strings.HasPrefix(line, "Buffers:"):
			buffers, err = parseMeminfoKBLine(line)
		case strings.HasPrefix(line, "Cached:"):
			cached, err = parseMeminfoKBLine(line)
		case strings.HasPrefix(line, "SReclaimable:"):
			sReclaimable, err = parseMeminfoKBLine(line)
		}
		if err != nil {
			return 0, 0, err
		}
	}
	if err = sc.Err(); err != nil {
		return 0, 0, err
	}
	totalBytes = memTotal * 1024
	if memAvailable > 0 {
		availableBytes = memAvailable * 1024
	} else {
		availKB := memFree + buffers + cached + sReclaimable
		if availKB > memTotal {
			availKB = memTotal
		}
		availableBytes = availKB * 1024
	}
	return totalBytes, availableBytes, nil
}

func parseMeminfoKBLine(line string) (uint64, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, nil
	}
	return strconv.ParseUint(fields[1], 10, 64)
}

func selfVmRSSKBytes() (uint64, error) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return 0, nil
			}
			return strconv.ParseUint(fields[1], 10, 64)
		}
	}
	return 0, sc.Err()
}
