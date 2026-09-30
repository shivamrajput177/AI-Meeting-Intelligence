package usecase

import (
	"fmt"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
)

// MaxDispatchAttempts caps how many times the poller retries one outbox
// row before giving up and marking it StatusFailed (publishing
// notification.failed.v1) — see repository/postgres's own doc comment on
// the backoff this pairs with.
const MaxDispatchAttempts = 5

// shouldGiveUp reports whether attempts (already incremented for the
// attempt that just failed) has exhausted MaxDispatchAttempts.
func shouldGiveUp(attempts int) bool {
	return attempts >= MaxDispatchAttempts
}

// actionItemStatusOpen/InProgress/Done mirror
// actionitemsvc/entity.StatusOpen/StatusInProgress/StatusDone —
// duplicated, not imported, matching every other cross-service constant
// in this repo (services never import each other's Go packages, only
// communicate over REST/Kafka).
const (
	actionItemStatusOpen       = "open"
	actionItemStatusInProgress = "in_progress"
	actionItemStatusDone       = "done"
)

// mapMockStatusToActionItemStatus is the mock board's own bidirectional
// sync mapping (see deployment-demo-strategy.md §3): a card drag to a
// given column maps onto the linked action item's own status the same
// way a real Jira transition webhook eventually would (Phase 4.3).
func mapMockStatusToActionItemStatus(mockStatus string) (string, bool) {
	switch mockStatus {
	case entity.MockStatusToDo:
		return actionItemStatusOpen, true
	case entity.MockStatusInProgress:
		return actionItemStatusInProgress, true
	case entity.MockStatusDone:
		return actionItemStatusDone, true
	default:
		return "", false
	}
}

// renderSummaryCompletedSlackText/renderSummaryReadyEmail are the message
// text for summary.completed.v1 — see
// docs/architecture/kafka-topics.md's flow-1 diagram:
// `NO-->>U: Slack "meeting summarized" + action items digest`.
func renderSummaryCompletedSlackText(meetingTitle string) string {
	return fmt.Sprintf("📝 %q has been summarized. The summary, key decisions, risks, and blockers are ready in the app.", meetingTitle)
}

func renderSummaryReadyEmail(meetingTitle string) (subject, body string) {
	subject = fmt.Sprintf("Your meeting summary is ready: %s", meetingTitle)
	body = fmt.Sprintf("The summary for %q is ready. Open the app to view the executive summary, key decisions, risks, and blockers.", meetingTitle)
	return subject, body
}

// renderActionItemDigestSlackText is action-item.extracted.v1's message
// text — the other half of the same diagram's "+ action items digest".
func renderActionItemDigestSlackText(itemCount int, meetingTitle string) string {
	noun := "action item"
	if itemCount != 1 {
		noun = "action items"
	}
	return fmt.Sprintf("✅ %d %s extracted from %q.", itemCount, noun, meetingTitle)
}
