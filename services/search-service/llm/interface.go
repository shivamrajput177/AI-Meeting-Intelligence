// Package llm defines the Embedder and Answerer interfaces usecase
// depends on; llm/ollama, right below this package in the same tree,
// implements both against a real Ollama server — see that package's doc
// comment. Keeping the interfaces here instead of off in some unrelated
// package is just where they belong — their one real implementation
// lives one directory down.
package llm

import "context"

// Embedder turns text into a fixed-length vector — see
// docs/architecture/microservices.md §9: "calls Ollama's embedding
// model". Used both to embed a chunk (Phase 3.2's write path) and to
// embed a search/RAG query (Phase 3.3/3.4's read path) — same model
// either way, since a query and the chunks it's compared against must
// share an embedding space.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Answerer runs the RAG chat completion — a grounded prompt (retrieved
// chunks + citations already assembled by the caller) in, a natural-
// language answer out. Separate from Embedder because they're typically
// different Ollama models (an embedding model can't do chat completion
// and vice versa).
type Answerer interface {
	Answer(ctx context.Context, question, groundedContext string) (string, error)
}
