package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/llm"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/metrics"
)

type AskUseCase struct {
	search    *SearchUseCase
	answerer  llm.Answerer
	repo      repository.Repository
	modelUsed string
}

func NewAskUseCase(search *SearchUseCase, answerer llm.Answerer, repo repository.Repository, modelUsed string) *AskUseCase {
	return &AskUseCase{search: search, answerer: answerer, repo: repo, modelUsed: modelUsed}
}

type AskResult struct {
	Answer    string
	Citations []Citation
}

type Citation struct {
	MeetingID    string
	MeetingTitle string
	StartMS      int
	EndMS        int
	Snippet      string
}

// Ask is POST /qa/ask's business logic — see
// docs/architecture/microservices.md §9's RAG flow: retrieve top-k chunks
// (reusing SearchUseCase's own embed-then-retrieve, since RAG retrieval
// *is* search, just with a different consumer of the results) → assemble
// a grounded prompt with [meeting_title, timestamp] citations inline →
// call Ollama for the answer → persist to qa_history.
//
// Citation simplification, stated plainly: every retrieved chunk is
// treated as a citation of the final answer. A real implementation might
// have the model name which specific excerpts it actually drew on, but
// that needs either function-calling or parsing citation markers back out
// of free-form prose — extra machinery this "basic RAG" pass skips in
// favor of a simpler, always-correct-if-slightly-generous guarantee: every
// citation shown genuinely was in the context the model saw.
func (uc *AskUseCase) Ask(ctx context.Context, orgID, userID, question string) (*AskResult, error) {
	start := time.Now()
	defer func() { metrics.RAGQueryDurationSeconds.Observe(time.Since(start).Seconds()) }()

	results, err := uc.search.Search(ctx, orgID, question)
	if err != nil {
		return nil, fmt.Errorf("retrieve context: %w", err)
	}

	groundedContext, citations, chunkIDs := buildContext(results)
	llmStart := time.Now()
	answer, err := uc.answerer.Answer(ctx, question, groundedContext)
	if err != nil {
		return nil, fmt.Errorf("ollama answer: %w", err)
	}
	metrics.LLMCallDurationSeconds.WithLabelValues(uc.modelUsed).Observe(time.Since(llmStart).Seconds())

	entry := &entity.QAHistoryEntry{
		ID: uuid.NewString(), OrgID: orgID, UserID: userID,
		Question: question, Answer: answer, CitedChunkIDs: chunkIDs, CreatedAt: time.Now(),
	}
	if err := uc.repo.SaveQAHistory(ctx, entry); err != nil {
		return nil, fmt.Errorf("save qa history: %w", err)
	}

	return &AskResult{Answer: answer, Citations: citations}, nil
}

func buildContext(results []SearchResult) (groundedContext string, citations []Citation, chunkIDs []string) {
	var b strings.Builder
	citations = make([]Citation, len(results))
	chunkIDs = make([]string, len(results))
	for i, r := range results {
		fmt.Fprintf(&b, "[%s, %s]: %s\n\n", r.MeetingTitle, formatTimestamp(r.Hit.StartMS), r.Hit.Text)
		citations[i] = Citation{
			MeetingID: r.Hit.MeetingID, MeetingTitle: r.MeetingTitle,
			StartMS: r.Hit.StartMS, EndMS: r.Hit.EndMS, Snippet: r.Hit.Text,
		}
		chunkIDs[i] = r.Hit.ChunkID
	}
	return b.String(), citations, chunkIDs
}

func formatTimestamp(ms int) string {
	totalSeconds := ms / 1000
	return fmt.Sprintf("%d:%02d", totalSeconds/60, totalSeconds%60)
}
