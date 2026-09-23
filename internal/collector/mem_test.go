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

func TestRamInfoFromAvailableMatchesFreeStyleAccounting(t *testing.T) {
	// Real-world sample captured via `free -h` vs /proc/meminfo on a test VPS:
	// free -h reported ~845-862MiB used while the old htop-style heuristic
	// (free+cached+buffers+sreclaimable) undercounted at ~663-675MiB. Used
	// must equal Total-Available, matching free -h's actual formula.
	info := &ProcMemInfo{
		MemTotal:     3081433088,
		MemFree:      1382776832,
		MemAvailable: 2189721600,
		Buffers:      62914560,
		Cached:       824180736,
		SReclaimable: 113246208,
	}
	got := ramInfoFromAvailable(info)
	want := info.MemTotal - info.MemAvailable
	if got.Used != want {
		t.Fatalf("Used = %d; want %d (Total-MemAvailable)", got.Used, want)
	}
	if got.Total != info.MemTotal {
		t.Fatalf("Total = %d; want %d", got.Total, info.MemTotal)
	}
}

func TestRamInfoFromAvailableMissingFieldsReturnsZero(t *testing.T) {
	if got := ramInfoFromAvailable(&ProcMemInfo{MemTotal: 1024}); got.Total != 0 {
		t.Fatalf("expected zero RAMInfo when MemAvailable is missing (old kernel), got %+v", got)
	}
	if got := ramInfoFromAvailable(&ProcMemInfo{MemAvailable: 512}); got.Total != 0 {
		t.Fatalf("expected zero RAMInfo when MemTotal is missing, got %+v", got)
	}
}

func TestRamInfoFromAvailableClampsWhenAvailableExceedsTotal(t *testing.T) {
	// Defensive: some cgroup-limited/virtualized environments have reported
	// MemAvailable slightly above MemTotal; Used must never underflow.
	got := ramInfoFromAvailable(&ProcMemInfo{MemTotal: 1024, MemAvailable: 2048})
	if got.Used != 0 {
		t.Fatalf("Used = %d; want 0 when MemAvailable >= MemTotal", got.Used)
	}
}
