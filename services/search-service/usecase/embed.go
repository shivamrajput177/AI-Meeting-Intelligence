package usecase

import (
	"context"
	"fmt"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/chunks"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/llm"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

type EmbedChunksUseCase struct {
	chunksClient chunks.Client
	embedder     llm.Embedder
	repo         repository.Repository
	publisher    events.Publisher
	log          *logger.Logger
	modelName    string
}

func NewEmbedChunksUseCase(
	chunksClient chunks.Client, embedder llm.Embedder, repo repository.Repository,
	publisher events.Publisher, log *logger.Logger, modelName string,
) *EmbedChunksUseCase {
	return &EmbedChunksUseCase{
		chunksClient: chunksClient, embedder: embedder, repo: repo,
		publisher: publisher, log: log, modelName: modelName,
	}
}

// EmbedChunks is chunk.created.v1's business logic (see consumer.go), and
// is also called directly by ReindexUseCase to re-embed one meeting on
// demand: fetch the meeting's chunk text from AI Summary Service (since
// chunk.created.v1 itself only carries a count — see
// entity.ChunkCreatedEvent's doc comment), embed each chunk via Ollama,
// and persist the batch. It deliberately doesn't publish a failure event
// itself — same reasoning as every other *ProcessX/*ExtractX usecase in
// this repo: consumer.go's retry loop owns deciding when an error is
// worth retrying versus when to give up and publish
// embedding.failed.v1, so that decision lives in exactly one place.
func (uc *EmbedChunksUseCase) EmbedChunks(ctx context.Context, orgID, meetingID string) ([]*entity.ChunkEmbedding, error) {
	chunkList, err := uc.chunksClient.ListChunks(ctx, orgID, meetingID)
	if err != nil {
		return nil, fmt.Errorf("fetch chunks: %w", err)
	}

	embeddings := make([]*entity.ChunkEmbedding, len(chunkList))
	for i, c := range chunkList {
		vec, err := uc.embedder.Embed(ctx, c.Text)
		if err != nil {
			return nil, fmt.Errorf("ollama embedding (chunk %s): %w", c.ID, err)
		}
		embeddings[i] = &entity.ChunkEmbedding{
			ChunkID: c.ID, MeetingID: meetingID, OrgID: orgID, Embedding: vec,
			ModelName: uc.modelName, MeetingText: c.Text, StartMS: c.StartMS, EndMS: c.EndMS,
		}
	}

	if err := uc.repo.ReplaceEmbeddings(ctx, orgID, meetingID, embeddings); err != nil {
		return nil, fmt.Errorf("store embeddings: %w", err)
	}

	if err := uc.publisher.PublishEmbeddingCompleted(ctx, entity.EmbeddingCompletedEvent{
		MeetingID: meetingID, OrgID: orgID, ChunkCount: len(embeddings),
	}); err != nil {
		// Best-effort, same trade-off as every other publish-after-persist
		// call in this repo: no transactional outbox yet, so a dropped
		// event here is a real, logged gap rather than a silently
		// swallowed one.
		uc.log.Error("publish embedding.completed.v1 failed", "meeting_id", meetingID, "err", err)
	}
	return embeddings, nil
}
