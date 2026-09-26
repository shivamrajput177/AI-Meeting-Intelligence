// Package llm defines the Extractor interface usecase depends on;
// llm/ollama, right below this package in the same tree, implements it
// against a real Ollama server — see that package's doc comment. Keeping
// the interface here instead of off in some unrelated package is just
// where it belongs — its one real implementation lives one directory
// down.
package llm

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
)

// Extractor runs the structured action-item extraction prompt against a
// meeting's summary text — see docs/architecture/microservices.md §8:
// "prompts the LLM with a structured-output schema to extract action
// items... with best-guess owners... assigns due dates when mentioned."
//
// Decisions/risks/blockers are deliberately NOT re-extracted here: the
// summary this service consumes already carries those as structured text
// (ai-summary-service's own SummaryResult), so ProcessSummaryUseCase
// turns those straight into actionitem rows without a second LLM call —
// this interface only covers the genuinely new extraction work
// (actionable items, with an owner guess and due date neither
// KeyDecisions/Risks/Blockers carries).
type Extractor interface {
	ExtractActionItems(ctx context.Context, summaryText string) ([]entity.ExtractedItem, error)
}
