package v2

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewRequestAndNotification(t *testing.T) {
	request, err := NewRequest("request-1", MethodAgentReport, map[string]string{"state": "ok"})
	if err != nil {
		t.Fatal(err)
	}
	var decoded Request
	if err := json.Unmarshal(request, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.JSONRPC != Version || decoded.ID != "request-1" || decoded.Method != MethodAgentReport {
		t.Fatalf("unexpected request: %#v", decoded)
	}

	notification, err := NewNotification(MethodAgentBasicInfo, map[string]string{"host": "agent"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(notification), `"id"`) {
		t.Fatalf("notification unexpectedly has an ID: %s", notification)
	}
}

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name, body string
		wantErr    bool
	}{
		{"success", `{"jsonrpc":"2.0","id":"request-1","result":{"ok":true}}`, false},
		{"rpc error", `{"jsonrpc":"2.0","id":"request-1","error":{"code":42,"message":"rejected"}}`, true},
		{"wrong version", `{"jsonrpc":"1.0","result":{}}`, true},
		{"malformed", `{`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseResponse([]byte(tt.body))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseResponse() error = %v, want error = %t", err, tt.wantErr)
			}
		})
	}
}

func TestBindParams(t *testing.T) {
	var target struct {
		Name string `json:"name"`
	}
	if err := BindParams(map[string]any{"name": "agent"}, &target); err != nil {
		t.Fatal(err)
	}
	if target.Name != "agent" {
		t.Fatalf("name = %q, want agent", target.Name)
	}
}
