package v2

import (
	"encoding/json"
	"fmt"
)

func NewNotification(method string, params any) []byte {
	payload, _ := json.Marshal(Request{JSONRPC: Version, Method: method, Params: params})
	return payload
}

func NewRequest(id any, method string, params any) []byte {
	payload, _ := json.Marshal(Request{JSONRPC: Version, Method: method, Params: params, ID: id})
	return payload
}

func BindParams(raw any, target any) error {
	payload, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, target)
}

func BindResult(raw any, target any) error {
	return BindParams(raw, target)
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
	const maxSnippetLength = 120
	if len(body) > maxSnippetLength {
		body = body[:maxSnippetLength]
	}
	return fmt.Sprintf("%q", string(body))
}
