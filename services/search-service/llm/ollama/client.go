// Package ollama implements llm.Embedder and llm.Answerer against a real
// Ollama server's REST API — /api/embeddings for Embedder,
// /api/generate for Answerer (same generate-with-a-prompt shape
// aisummarysvc/llm/ollama and actionitemsvc/llm/ollama already use, just
// without "format": "json" since a RAG answer is prose, not a structured
// object).
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// EmbedClient embeds text via a fixed model (e.g. "nomic-embed-text") —
// see docs/PROJECT_PLAN.md's tech-stack row for why that model: 768
// dimensions, matching search.chunk_embeddings.embedding's column width.
type EmbedClient struct {
	baseURL string
	model   string
	http    *http.Client
}

func NewEmbedClient(baseURL, model string) *EmbedClient {
	return &EmbedClient{baseURL: baseURL, model: model, http: &http.Client{Timeout: 2 * time.Minute}}
}

type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

func (c *EmbedClient) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody, err := json.Marshal(embedRequest{Model: c.model, Prompt: text})
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embeddings", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("ollama: build embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: embed request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: embed server returned %d: %s", resp.StatusCode, string(b))
	}

	var parsed embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("ollama: decode embed response: %w", err)
	}
	if len(parsed.Embedding) == 0 {
		return nil, fmt.Errorf("ollama: embed response had no embedding")
	}
	return parsed.Embedding, nil
}

// AnswerClient runs the RAG chat completion via a fixed model (the same
// LLM AI Summary/Action Item Services already use — no reason to pull a
// second chat model just for this).
type AnswerClient struct {
	baseURL string
	model   string
	http    *http.Client
}

func NewAnswerClient(baseURL, model string) *AnswerClient {
	return &AnswerClient{baseURL: baseURL, model: model, http: &http.Client{Timeout: 5 * time.Minute}}
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
}

func buildPrompt(question, groundedContext string) string {
	return "You are an assistant answering a question about a team's past meetings, using only the excerpts below. " +
		"If the excerpts don't contain enough information to answer, say so plainly rather than guessing. " +
		"Answer in plain prose, no markdown, no citation markers — the excerpts' sources are already tracked separately.\n\n" +
		"Meeting excerpts:\n" + groundedContext +
		"\n\nQuestion: " + question
}

func (c *AnswerClient) Answer(ctx context.Context, question, groundedContext string) (string, error) {
	reqBody, err := json.Marshal(generateRequest{
		Model: c.model, Prompt: buildPrompt(question, groundedContext), Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("ollama: marshal generate request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("ollama: build generate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: generate request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ollama: generate server returned %d: %s", resp.StatusCode, string(b))
	}

	var parsed generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("ollama: decode generate response: %w", err)
	}
	return parsed.Response, nil
}
