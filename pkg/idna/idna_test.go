package idna

import "testing"

func TestConvertIDNToASCII(t *testing.T) {
	tests := []struct{ name, input, want string }{
		{"unicode host", "https://中文域名.com:8443/path?q=1", "https://xn--fiq06l2rdsvs.com:8443/path?q=1"},
		{"ipv4", "https://127.0.0.1:8080/path", "https://127.0.0.1:8080/path"},
		{"ipv6", "https://[2001:db8::1]:8080/path", "https://[2001:db8::1]:8080/path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertIDNToASCII(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("ConvertIDNToASCII() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertIDNToASCIIRejectsInvalidURL(t *testing.T) {
	if _, err := ConvertIDNToASCII("https://[::1"); err == nil {
		t.Fatal("invalid URL was accepted")
	}
}
