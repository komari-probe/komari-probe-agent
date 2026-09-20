package netstatic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestTrackerPersistsNICConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.json")
	tracker := NewTracker(path)
	if err := tracker.ConfigureNICs([]string{"eth0", "wg0"}); err != nil {
		t.Fatalf("ConfigureNICs() error = %v", err)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted tracker: %v", err)
	}
	var persisted persistedData
	if err := json.Unmarshal(contents, &persisted); err != nil {
		t.Fatalf("decode persisted tracker: %v", err)
	}
	if want := []string{"eth0", "wg0"}; !reflect.DeepEqual(persisted.Config.NICs, want) {
		t.Fatalf("persisted NICs = %v, want %v", persisted.Config.NICs, want)
	}
}

func TestTrackerReadsExistingHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "traffic.json")
	now := uint64(time.Now().Unix())
	persisted := persistedData{
		Interfaces: map[string][]TrafficData{
			"eth0": {{Timestamp: now, Tx: 12, Rx: 34}},
		},
		Config: Config{DataPreserveDays: 1, DetectInterval: 60, SaveInterval: 60},
	}
	contents, err := json.Marshal(persisted)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	tracker := NewTracker(path)
	if err := tracker.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer tracker.Close()

	totals, err := tracker.TotalTrafficBetween(now, now)
	if err != nil {
		t.Fatalf("TotalTrafficBetween() error = %v", err)
	}
	if got := totals["eth0"]; got.Tx != 12 || got.Rx != 34 {
		t.Fatalf("eth0 total = %#v, want Tx=12 Rx=34", got)
	}
}
