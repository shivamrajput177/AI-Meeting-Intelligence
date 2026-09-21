// Package llm defines the Summarizer interface usecase depends on;
// llm/ollama, right below this package in the same tree, implements it
// against a real Ollama server — see that package's doc comment. Keeping
// the interface here instead of off in some unrelated package is just
// where it belongs — its one real implementation lives one directory
// down.
package llm

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
)

// Summarizer runs the structured summarization prompt against a
// transcript's raw text — see
// docs/architecture/microservices.md §7's "summarization pipeline via
// Ollama (structured prompt -> executive summary, key decisions, risks,
// blockers)".
type Summarizer interface {
	Summarize(ctx context.Context, transcriptText string) (*entity.SummaryResult, error)
}
