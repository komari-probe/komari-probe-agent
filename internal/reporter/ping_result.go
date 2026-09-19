package reporter

import (
	"log"
	"time"

	"github.com/komari-probe/komari-probe-agent/internal/connectivity"
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
	payload := buildPingResultPayload(taskID, pingType, pingResult, time.Now())
	if err := sendRPC(conn, payload); err != nil {
		log.Printf("Failed to send ping result: %v", err)
	}
}
