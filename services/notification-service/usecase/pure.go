package usecase

import "fmt"

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
