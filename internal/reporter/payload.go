package reporter

import (
	"encoding/json"
	"time"

	v2 "github.com/sonar-probe/sonar-agent/internal/rpc/v2"
)

type reportParams struct {
	Report      json.RawMessage `json:"report"`
	AckEventIDs []string        `json:"ack_event_ids,omitempty"`
}

func buildReportPayload(report []byte) ([]byte, error) {
	return v2.NewNotification(v2.MethodAgentReport, reportParams{Report: json.RawMessage(report)})
}

func buildReportRequest(id any, report []byte, ackEventIDs []string) ([]byte, error) {
	return v2.NewRequest(id, v2.MethodAgentReport, reportParams{Report: json.RawMessage(report), AckEventIDs: ackEventIDs})
}

func buildBasicInfoPayload(info map[string]any) ([]byte, error) {
	return v2.NewNotification(v2.MethodAgentBasicInfo, map[string]any{"info": info})
}

func buildPingResultPayload(taskID uint, pingType string, value int, finishedAt time.Time) any {
	return v2.Request{
		JSONRPC: v2.Version,
		Method:  v2.MethodAgentPingResult,
		Params: map[string]any{
			"task_id":     taskID,
			"ping_type":   pingType,
			"value":       value,
			"finished_at": finishedAt.Format(time.RFC3339Nano),
		},
	}
}
