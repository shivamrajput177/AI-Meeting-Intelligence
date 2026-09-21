// Package ollama implements llm.Summarizer against a real Ollama server's
// /api/generate REST endpoint (Ollama's own documented API — see
// docs/architecture/microservices.md §7 and PROJECT_PLAN.md §3's tech
// stack row: "Ollama - Serves Llama 3 8B / Qwen2.5 7B / Mistral 7B").
// Ollama loads the model named in each request itself (unlike
// whisper.cpp, which fixes one model per server process), so the model
// name is a per-request field here, not a client constructor argument.
//
// Structured output is requested via Ollama's "format": "json" mode
// (constrains the model's output to valid JSON) plus a prompt that
// spells out the exact keys wanted, then decoded straight into
// entity.SummaryResult's shape — no separate JSON schema/function-calling
// layer, matching this project's "no schema registry, no codegen"
// REST/Kafka payload philosophy elsewhere.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
)

const promptVersion = "v1"

// PromptVersion is stored on every entity.Summary row (see
// docs/architecture/microservices.md §7's ai.summaries.prompt_version
// column) so a later prompt change is visible in the data, not just in
// git history.
func PromptVersion() string { return promptVersion }

type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

// New builds a client against a running Ollama server for the given
// model (e.g. "qwen2.5:7b") — Ollama pulls/loads it on first use if it
// isn't already resident, per Ollama's own documented behavior.
func New(baseURL, model string) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		// LLM inference on CPU for a full meeting transcript can
		// legitimately take minutes — same rationale as
		// asr/whispercpp.Client's timeout.
		http: &http.Client{Timeout: 10 * time.Minute},
	}
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format"`
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// summaryJSON is the exact shape the prompt below asks the model to
// return — decoded straight out of generateResponse.Response.
type summaryJSON struct {
	Summary      string   `json:"summary"`
	KeyDecisions []string `json:"keyDecisions"`
	Risks        []string `json:"risks"`
	Blockers     []string `json:"blockers"`
}

func buildPrompt(transcriptText string) string {
	return "You are an assistant that summarizes meeting transcripts for a busy team. " +
		"Read the transcript below and respond with ONLY a JSON object (no markdown, no commentary) " +
		"with exactly these keys: " +
		`"summary" (a concise executive summary, 2-4 sentences), ` +
		`"keyDecisions" (an array of strings, one per decision actually made — empty array if none), ` +
		`"risks" (an array of strings describing risks or concerns raised — empty array if none), ` +
		`"blockers" (an array of strings describing blockers mentioned — empty array if none).` +
		"\n\nTranscript:\n" + transcriptText
}

func (c *Client) Summarize(ctx context.Context, transcriptText string) (*entity.SummaryResult, error) {
	reqBody, err := json.Marshal(generateRequest{
		Model: c.model, Prompt: buildPrompt(transcriptText), Stream: false, Format: "json",
	})
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("ollama: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama: server returned %d: %s", resp.StatusCode, string(b))
	}

	var gen generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&gen); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}

	var parsed summaryJSON
	if err := json.Unmarshal([]byte(gen.Response), &parsed); err != nil {
		return nil, fmt.Errorf("ollama: model did not return valid JSON: %w", err)
	}

	return &entity.SummaryResult{
		SummaryText:  parsed.Summary,
		KeyDecisions: parsed.KeyDecisions,
		Risks:        parsed.Risks,
		Blockers:     parsed.Blockers,
	}, nil
}
