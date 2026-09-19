package gpu

import (
	"testing"
)

func TestName(t *testing.T) {
	name := Name()
	if name == "" || name == "Unknown" {
		t.Errorf("Expected GPU name, got empty or 'Unknown'")
	}
	t.Logf("GPU name: %s", name)
}

func TestFormatGPUNameList(t *testing.T) {
	got := formatNameList([]string{
		"NVIDIA GeForce RTX 4090",
		"NVIDIA GeForce RTX 4090",
		"AMD Radeon RX 7900 XTX",
	})
	want := "NVIDIA GeForce RTX 4090 × 2, AMD Radeon RX 7900 XTX"
	if got != want {
		t.Fatalf("formatNameList() = %q, want %q", got, want)
	}
}
