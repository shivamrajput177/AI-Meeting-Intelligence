// poller.go ticks DispatchUseCase — the poller half of the transactional
// outbox pattern (see entity.OutboxRow's doc comment). Any replica of
// this service can run it: ClaimBatch's `FOR UPDATE SKIP LOCKED` is what
// makes running it on every replica safe rather than needing a
// leader-elected singleton the way Phase 4.4's reminder scheduler will.
package main

import (
	"context"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

const (
	pollerInterval  = 5 * time.Second
	pollerBatchSize = 20
)

// RunDispatchPoller runs until ctx is cancelled, dispatching one batch per
// tick — a claim/dispatch error is logged and retried on the next tick
// rather than crashing the process, since it's typically Slack/SMTP being
// transiently unreachable, exactly the case retry-with-backoff exists for.
func RunDispatchPoller(ctx context.Context, dispatch *usecase.DispatchUseCase, log *logger.Logger) {
	ticker := time.NewTicker(pollerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := dispatch.DispatchBatch(ctx, pollerBatchSize)
			if err != nil {
				log.Error("dispatch batch", "err", err)
				continue
			}
			if n > 0 {
				log.Info("dispatched batch", "count", n)
			}
		}
	}
}
