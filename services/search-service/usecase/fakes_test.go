package usecase_test

import (
	"context"
	"sync"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
)

// fakeChunksClient, fakeMeetingsClient, fakeEmbedder, fakeAnswerer,
// fakeRepository, and fakePublisher are in-memory stand-ins for the real
// AI Summary Service HTTP client / Meeting Service HTTP client / Ollama
// embed client / Ollama chat client / Postgres repository / Kafka
// producer — this is the whole point of defining each as an interface
// usecase depends on (see docs/architecture/folder-structure.md's Clean
// Architecture layering): every usecase in this package is fully
// testable with none of those real systems involved.
type fakeChunksClient struct {
	chunks map[string][]entity.Chunk // meetingID -> chunks
	err    error
}

func (f *fakeChunksClient) ListChunks(_ context.Context, _, meetingID string) ([]entity.Chunk, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.chunks[meetingID], nil
}

type fakeMeetingsClient struct {
	titles   map[string]string // meetingID -> title
	meetings []entity.Meeting
	getErr   error
	listErr  error
}

func (f *fakeMeetingsClient) GetMeeting(_ context.Context, _, meetingID string) (*entity.Meeting, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &entity.Meeting{ID: meetingID, Title: f.titles[meetingID]}, nil
}

func (f *fakeMeetingsClient) ListMeetings(_ context.Context, _ string, _, _ int) ([]entity.Meeting, int, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	return f.meetings, len(f.meetings), nil
}

type fakeEmbedder struct {
	vec []float32
	err error
}

func (f *fakeEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vec, nil
}

type fakeAnswerer struct {
	answer string
	err    error
}

func (f *fakeAnswerer) Answer(_ context.Context, _, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.answer, nil
}

type fakeRepository struct {
	mu         sync.Mutex
	embeddings map[string][]*entity.ChunkEmbedding // meetingID -> embeddings
	searchHits []*entity.SearchHit
	similar    []*entity.SimilarMeeting
	qaHistory  []*entity.QAHistoryEntry
}

func (f *fakeRepository) ReplaceEmbeddings(_ context.Context, _, meetingID string, embeddings []*entity.ChunkEmbedding) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.embeddings == nil {
		f.embeddings = map[string][]*entity.ChunkEmbedding{}
	}
	f.embeddings[meetingID] = embeddings
	return nil
}

func (f *fakeRepository) GetEmbeddingsByMeeting(_ context.Context, _, meetingID string) ([]*entity.ChunkEmbedding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.embeddings[meetingID], nil
}

func (f *fakeRepository) SemanticSearch(_ context.Context, _ string, _ []float32, _ int) ([]*entity.SearchHit, error) {
	return f.searchHits, nil
}

func (f *fakeRepository) SimilarMeetings(_ context.Context, _, _ string, _ []float32, _ int) ([]*entity.SimilarMeeting, error) {
	return f.similar, nil
}

func (f *fakeRepository) SaveQAHistory(_ context.Context, entry *entity.QAHistoryEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.qaHistory = append(f.qaHistory, entry)
	return nil
}

func (f *fakeRepository) ListQAHistory(_ context.Context, _, _ string) ([]*entity.QAHistoryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.qaHistory, nil
}

type fakePublisher struct {
	mu        sync.Mutex
	completed []entity.EmbeddingCompletedEvent
	failed    []entity.EmbeddingFailedEvent
}

func (f *fakePublisher) PublishEmbeddingCompleted(_ context.Context, event entity.EmbeddingCompletedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completed = append(f.completed, event)
	return nil
}

func (f *fakePublisher) PublishEmbeddingFailed(_ context.Context, event entity.EmbeddingFailedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = append(f.failed, event)
	return nil
}
