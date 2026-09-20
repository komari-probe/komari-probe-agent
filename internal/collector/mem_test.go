package collector

import (
	"strings"
	"testing"
)

func TestParseProcMeminfo(t *testing.T) {
	info, err := parseProcMeminfo(strings.NewReader(`
MemTotal:       1024 kB
MemFree:         128 kB
MemAvailable:    256 kB
Buffers:          16 kB
Cached:           32 kB
SwapTotal:       512 kB
SwapFree:        400 kB
SwapCached:       12 kB
Shmem:             8 kB
SReclaimable:       4 kB
Zswap:              2 kB
Zswapped:           1 kB
Unrelated:         99 kB
`))
	if err != nil {
		t.Fatalf("parseProcMeminfo() error = %v", err)
	}
	if info.MemTotal != 1024*1024 || info.MemAvailable != 256*1024 {
		t.Fatalf("memory values = total %d, available %d; want 1048576, 262144", info.MemTotal, info.MemAvailable)
	}
	if info.SwapTotal != 512*1024 || info.SwapCached != 12*1024 {
		t.Fatalf("swap values = total %d, cached %d; want 524288, 12288", info.SwapTotal, info.SwapCached)
	}
	if info.Zswap != 2*1024 || info.Zswapped != 1024 {
		t.Fatalf("zswap values = %d/%d; want 2048/1024", info.Zswap, info.Zswapped)
	}
}

func TestParseProcMeminfoSkipsMalformedValues(t *testing.T) {
	info, err := parseProcMeminfo(strings.NewReader("MemTotal: invalid kB\nMemFree:\nCached: 10 kB\n"))
	if err != nil {
		t.Fatalf("parseProcMeminfo() error = %v", err)
	}
	if info.MemTotal != 0 || info.MemFree != 0 || info.Cached != 10*1024 {
		t.Fatalf("parsed values = %+v; malformed entries should be ignored", info)
	}
}
