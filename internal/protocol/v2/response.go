package v2

import (
	"encoding/json"
	"fmt"
)

type HTTPStatusError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return ""
	}
	if e.Body != "" {
		return fmt.Sprintf("status code: %d,%s", e.StatusCode, e.Body)
	}
	if e.Status != "" {
		return e.Status
	}
	return fmt.Sprintf("status code: %d", e.StatusCode)
}

func ParseResponse(body []byte) (*Response, error) {
	var rpcResp Response
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("invalid v2 JSON-RPC response: %w, body: %s", err, bodySnippet(body))
	}
	if rpcResp.JSONRPC != Version {
		return nil, fmt.Errorf("invalid v2 JSON-RPC version %q, body: %s", rpcResp.JSONRPC, bodySnippet(body))
	}
	if rpcResp.Error != nil {
		return &rpcResp, fmt.Errorf("v2 rpc error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return &rpcResp, nil
}

func bodySnippet(body []byte) string {
	const max = 120
	if len(body) > max {
		body = body[:max]
	}
	return fmt.Sprintf("%q", string(body))
}
