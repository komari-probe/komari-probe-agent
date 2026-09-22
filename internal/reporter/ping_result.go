package reporter

import (
	"context"
	log "github.com/sonar-probe/sonar-agent/internal/logging"
	"time"

	"github.com/sonar-probe/sonar-agent/internal/connectivity"
	"github.com/sonar-probe/sonar-agent/internal/task"
)

func (r *Reporter) reportPingTask(ctx context.Context, conn *connectivity.SafeConn, taskID uint, pingType, pingTarget string) {
	if taskID == 0 {
		log.Printf("Invalid task ID: %d", taskID)
		return
	}

	pingResult, err := task.Probe(ctx, pingType, pingTarget)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		log.Printf("Ping task %d failed: %v", taskID, err)
	}
	if ctx.Err() != nil {
		return
	}
	payload := buildPingResultPayload(taskID, pingType, pingResult, time.Now())
	if err := r.sendRPC(ctx, conn, payload); err != nil {
		log.Printf("Failed to send ping result: %v", err)
	}
}
