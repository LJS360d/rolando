//go:build !linux

package hostmem

type Stats struct {
	HostTotalBytes     uint64
	HostAvailableBytes uint64
	ProcessRSSBytes    uint64
}

func ReadStats() (Stats, error) {
	return Stats{}, nil
}
