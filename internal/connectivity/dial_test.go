package connectivity

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestNormalizeIPVersionPreference(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"4", "4"}, {"6", "6"}, {"", ""}, {"ipv4", ""},
	} {
		if got := normalizeIPVersionPreference(test.input); got != test.want {
			t.Errorf("normalizeIPVersionPreference(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestDialContextRejectsInvalidAddress(t *testing.T) {
	dial := NewManager(Options{}).NewDialContextWithPreference(time.Second, "4")
	if _, err := dial(context.Background(), "tcp", "missing-port"); err == nil {
		t.Fatal("dial() accepted an address without a port")
	}
}

func TestDialContextConnectsToLocalListener(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	accepted := make(chan struct{})
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = connection.Close()
			close(accepted)
		}
	}()

	dial := NewManager(Options{}).NewDialContextWithPreference(time.Second, "4")
	connection, err := dial(context.Background(), "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial() error = %v", err)
	}
	_ = connection.Close()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("listener did not accept the connection")
	}
}
