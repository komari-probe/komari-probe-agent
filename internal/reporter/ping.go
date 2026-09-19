package reporter

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
	"github.com/komari-probe/komari-probe-agent/internal/protocol/transport"
	v2 "github.com/komari-probe/komari-probe-agent/internal/protocol/v2"
	"github.com/komari-probe/komari-probe-agent/internal/task"
)

func reportPingTask(conn *connectivity.SafeConn, taskID uint, pingType, pingTarget string) {
	if taskID == 0 {
		log.Printf("Invalid task ID: %d", taskID)
		return
	}

	pingResult, err := task.Probe(pingType, pingTarget)
	if err != nil {
		log.Printf("Ping task %d failed: %v", taskID, err)
	}
	payload := v2.BuildPingResultPayload(taskID, pingType, pingResult, time.Now())
	if conn == nil {
		if err := postV2RPC(payload); err != nil {
			log.Printf("Failed to upload ping result over POST: %v", err)
		}
		return
	}
	if err := conn.WriteJSON(payload); err != nil {
		log.Printf("Failed to write JSON to WebSocket: %v", err)
	}
}

func postV2RPC(payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := strings.TrimSuffix(flags.Endpoint, "/") + "/api/clients/v2/rpc?token=" + flags.Token
	compressed := false
	if !flags.DisableCompression {
		if gz, err := transport.GzipBytes(body); err == nil {
			body = gz
			compressed = true
		}
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}
	client := connectivity.GetHTTPClientWithPreference(30*time.Second, flags.PreferIPVersion, flags.IgnoreUnsafeCert)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return &v2.HTTPStatusError{StatusCode: resp.StatusCode, Status: resp.Status, Body: string(respBody)}
	}
	if len(bytes.TrimSpace(respBody)) > 0 {
		if _, err := v2.ParseResponse(respBody); err != nil {
			return err
		}
	}
	return nil
}
