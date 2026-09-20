package logging

import "testing"

func TestSetLevel(t *testing.T) {
	t.Cleanup(func() { _ = SetLevel("info") })
	for _, value := range []string{"debug", "info", "warn", "warning", "error", "INFO"} {
		if err := SetLevel(value); err != nil {
			t.Fatalf("SetLevel(%q) error = %v", value, err)
		}
	}
	if err := SetLevel("verbose"); err == nil {
		t.Fatal("SetLevel() accepted an unsupported level")
	}
}
