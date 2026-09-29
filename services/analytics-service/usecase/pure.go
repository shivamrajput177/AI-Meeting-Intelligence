package usecase

import (
	"strings"
	"time"
)

// dayOf truncates t to its UTC calendar date at midnight — every rollup
// usecase buckets by the Kafka broker timestamp of the event that produced
// it (see entity.MeetingDailyRollup's doc comment on why, not
// time.Now()), and this is the one place that timestamp gets turned into
// the DATE a Postgres row keys on.
func dayOf(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// weekOf truncates t to the UTC date of the Monday starting its ISO week
// — analytics.topic_frequency buckets by week, not day, since a single
// meeting's topics are too sparse a signal to trend day-by-day.
func weekOf(t time.Time) time.Time {
	day := dayOf(t)
	offset := (int(day.Weekday()) + 6) % 7 // Monday=0 .. Sunday=6
	return day.AddDate(0, 0, -offset)
}

// extractTopics is the topic-frequency rollup's keyword-proxy extraction:
// each of a summary's key decisions/risks/blockers becomes its own topic
// string, lowercased and trimmed, empty phrases dropped. This is
// deliberately not real NLP keyword/entity extraction (no noun-phrase
// chunking, no stopword filtering, no clustering of near-duplicate
// phrases) — see entity.TopicFrequency's doc comment for the full
// reasoning. It's a real signal (an actual phrase the LLM summarizer
// produced, not a stub), just a coarse one: "the team needs to finalize
// the vendor contract by Friday" becomes one topic, not "vendor contract".
func extractTopics(keyDecisions, risks, blockers []string) []string {
	var topics []string
	for _, group := range [][]string{keyDecisions, risks, blockers} {
		for _, phrase := range group {
			t := strings.ToLower(strings.TrimSpace(phrase))
			if t != "" {
				topics = append(topics, t)
			}
		}
	}
	return topics
}
