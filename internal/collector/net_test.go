package collector

import (
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestConnectionsCount(t *testing.T) {
	tcpCount, udpCount, err := New(Options{}).ConnectionsCount()
	if err != nil {
		t.Fatalf("ConnectionsCount failed: %v", err)
	}

	if tcpCount < 0 {
		t.Errorf("Expected non-negative TCP count, got %d", tcpCount)
	}

	if udpCount < 0 {
		t.Errorf("Expected non-negative UDP count, got %d", udpCount)
	}

	t.Logf("TCP connections: %d, UDP connections: %d", tcpCount, udpCount)
}

func TestConnectionsCountCombinesProcAndFallbackErrors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("proc net fast path only runs on linux")
	}

	fallbackErr := errors.New("fallback unavailable")
	fallback := func() (int, int, error) {
		return 0, 0, fallbackErr
	}

	tcpCount, udpCount, err := connectionsCountWithProcFallback(t.TempDir(), fallback)
	if err == nil {
		t.Fatal("expected combined error, got nil")
	}
	if tcpCount != 0 || udpCount != 0 {
		t.Fatalf("expected zero counts on failure, got tcp=%d udp=%d", tcpCount, udpCount)
	}
	if !errors.Is(err, fallbackErr) {
		t.Fatalf("expected combined error to wrap fallback error, got %v", err)
	}

	errText := err.Error()
	for _, want := range []string{"proc net fast path failed", "no proc net files found", "gopsutil fallback failed"} {
		if !strings.Contains(errText, want) {
			t.Fatalf("expected error %q to contain %q", errText, want)
		}
	}
}

func TestParseNICs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]struct{}
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:  "single nic",
			input: "eth0",
			expected: map[string]struct{}{
				"eth0": {},
			},
		},
		{
			name:  "multiple nics",
			input: "eth0,wlan0,enp0s3",
			expected: map[string]struct{}{
				"eth0":   {},
				"wlan0":  {},
				"enp0s3": {},
			},
		},
		{
			name:  "nics with spaces",
			input: " eth0 , wlan0 , enp0s3 ",
			expected: map[string]struct{}{
				"eth0":   {},
				"wlan0":  {},
				"enp0s3": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNICs(tt.input)

			if tt.expected == nil && result != nil {
				t.Errorf("Expected nil, got %v", result)
				return
			}

			if tt.expected != nil && result == nil {
				t.Errorf("Expected %v, got nil", tt.expected)
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d", len(tt.expected), len(result))
				return
			}

			for key := range tt.expected {
				if _, exists := result[key]; !exists {
					t.Errorf("Expected key %s not found in result", key)
				}
			}
		})
	}
}

func TestShouldInclude(t *testing.T) {
	tests := []struct {
		name        string
		nicName     string
		includeNICs map[string]struct{}
		excludeNICs map[string]struct{}
		expected    bool
	}{
		{
			name:        "loopback interface should be excluded",
			nicName:     "lo",
			includeNICs: nil,
			excludeNICs: nil,
			expected:    false,
		},
		{
			name:        "docker interface should be excluded",
			nicName:     "docker0",
			includeNICs: nil,
			excludeNICs: nil,
			expected:    false,
		},
		{
			name:        "normal interface with no filters",
			nicName:     "eth0",
			includeNICs: nil,
			excludeNICs: nil,
			expected:    true,
		},
		{
			name:    "interface in include list",
			nicName: "eth0",
			includeNICs: map[string]struct{}{
				"eth0": {},
			},
			excludeNICs: nil,
			expected:    true,
		},
		{
			name:    "interface not in include list",
			nicName: "wlan0",
			includeNICs: map[string]struct{}{
				"eth0": {},
			},
			excludeNICs: nil,
			expected:    false,
		},
		{
			name:        "interface in exclude list",
			nicName:     "eth0",
			includeNICs: nil,
			excludeNICs: map[string]struct{}{
				"eth0": {},
			},
			expected: false,
		},
		{
			name:        "interface not in exclude list",
			nicName:     "wlan0",
			includeNICs: nil,
			excludeNICs: map[string]struct{}{
				"eth0": {},
			},
			expected: true,
		},
		{
			name:    "loopback in include list should still be excluded",
			nicName: "lo",
			includeNICs: map[string]struct{}{
				"lo": {},
			},
			excludeNICs: nil,
			expected:    false,
		},
		{
			name:        "interface with wildcard pattern in exclude list",
			nicName:     "tun0",
			includeNICs: nil,
			excludeNICs: map[string]struct{}{
				"tun*": {},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldInclude(tt.nicName, tt.includeNICs, tt.excludeNICs)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNetworkSpeedFallback(t *testing.T) {
	// 测试回退方法
	includeNICs := map[string]struct{}{}
	excludeNICs := map[string]struct{}{}

	totalUp, totalDown, upSpeed, downSpeed, err := New(Options{}).getNetworkSpeedFallback(includeNICs, excludeNICs)
	if err != nil {
		t.Fatalf("getNetworkSpeedFallback failed: %v", err)
	}

	t.Logf("TotalUp: %d, TotalDown: %d, UpSpeed: %d/s, DownSpeed: %d/s",
		totalUp, totalDown, upSpeed, downSpeed)
}

func TestNetworkSpeedSamplesAreCollectorScoped(t *testing.T) {
	firstCollector := New(Options{})
	secondCollector := New(Options{})
	start := time.Unix(1_000, 0)

	if up, down := firstCollector.updateNetworkSpeedSample(100, 200, start); up != 0 || down != 0 {
		t.Fatalf("first collector initial speed = %d/%d, want 0/0", up, down)
	}
	if up, down := secondCollector.updateNetworkSpeedSample(100, 200, start); up != 0 || down != 0 {
		t.Fatalf("second collector inherited first collector state: %d/%d", up, down)
	}
	if up, down := firstCollector.updateNetworkSpeedSample(110, 220, start.Add(time.Second)); up != 10 || down != 20 {
		t.Fatalf("first collector speed = %d/%d, want 10/20", up, down)
	}
}

func TestNetworkSpeedWithoutMonthRotate(t *testing.T) {
	hostCollector := New(Options{MonthRotate: 1})
	totalUp, totalDown, upSpeed, downSpeed, err := hostCollector.NetworkSpeed()
	if err != nil {
		t.Fatalf("NetworkSpeed failed: %v", err)
	}

	t.Logf("Without MonthRotate - TotalUp: %d, TotalDown: %d, UpSpeed: %d/s, DownSpeed: %d/s",
		totalUp, totalDown, upSpeed, downSpeed)
}

func TestNetworkSpeedWithMonthRotate(t *testing.T) {
	hostCollector := New(Options{MonthRotate: 1})
	totalUp, totalDown, upSpeed, downSpeed, err := hostCollector.NetworkSpeed()

	// 如果vnstat不可用，可能会回退到原来的方法，这是正常的
	if err != nil {
		t.Fatalf("NetworkSpeed failed: %v", err)
	}

	t.Logf("With MonthRotate - TotalUp: %d, TotalDown: %d, UpSpeed: %d/s, DownSpeed: %d/s",
		totalUp, totalDown, upSpeed, downSpeed)
}

func TestNetworkSpeedWithNicFilters(t *testing.T) {
	hostCollector := New(Options{ExcludeNICs: "lo,docker0"})
	totalUp, totalDown, upSpeed, downSpeed, err := hostCollector.NetworkSpeed()
	if err != nil {
		t.Fatalf("NetworkSpeed with excludeNICs failed: %v", err)
	}

	t.Logf("With excludeNICs - TotalUp: %d, TotalDown: %d, UpSpeed: %d/s, DownSpeed: %d/s",
		totalUp, totalDown, upSpeed, downSpeed)
}
