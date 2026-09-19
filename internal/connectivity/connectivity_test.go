package connectivity

import (
	"reflect"
	"testing"
)

func TestNormalizeDNSServer(t *testing.T) {
	tests := map[string]string{
		"1.1.1.1":                   "1.1.1.1:53",
		"1.1.1.1:5353":              "1.1.1.1:5353",
		"2001:4860:4860::8888":      "[2001:4860:4860::8888]:53",
		"[2001:4860:4860::8888]:53": "[2001:4860:4860::8888]:53",
	}
	for input, want := range tests {
		if got := normalizeDNSServer(input); got != want {
			t.Errorf("normalizeDNSServer(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSortIPsByPreference(t *testing.T) {
	ips := []string{"2001:db8::1", "192.0.2.1", "2001:db8::2", "192.0.2.2"}
	sortIPsByPreference(ips, "4")
	want := []string{"192.0.2.1", "192.0.2.2", "2001:db8::1", "2001:db8::2"}
	if !reflect.DeepEqual(ips, want) {
		t.Errorf("sortIPsByPreference() = %v, want %v", ips, want)
	}
}
