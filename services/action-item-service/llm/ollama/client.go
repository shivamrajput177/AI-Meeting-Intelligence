// Package ollama implements llm.Extractor against a real Ollama server's
// /api/generate REST endpoint — same client shape as
// aisummarysvc/llm/ollama (structured output via "format": "json" plus a
// prompt spelling out the exact keys wanted, decoded straight into
// entity.ExtractedItem's shape — no separate JSON schema/function-calling
// layer, matching this project's "no schema registry, no codegen"
// REST/Kafka payload philosophy elsewhere).
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
)

const promptVersion = "v1"

// PromptVersion mirrors aisummarysvc/llm/ollama.PromptVersion's rationale
// — kept here in case a future ai.summaries-style column ever wants to
// track which prompt version produced a given batch of action items.
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
		// LLM inference on CPU can legitimately take minutes — same
		// rationale as aisummarysvc/llm/ollama.Client's timeout.
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

type extractedItemJSON struct {
	Description string  `json:"description"`
	Owner       string  `json:"owner"`
	DueDate     string  `json:"dueDate"`
	Priority    string  `json:"priority"`
	Confidence  float32 `json:"confidence"`
}

// actionItemsJSON is the exact shape the prompt below asks the model to
// return — decoded straight out of generateResponse.Response.
type actionItemsJSON struct {
	ActionItems []extractedItemJSON `json:"actionItems"`
}

func buildPrompt(summaryText string) string {
	return "You are an assistant that extracts concrete action items from a meeting summary for a busy team. " +
		"Read the summary below and respond with ONLY a JSON object (no markdown, no commentary) " +
		`with exactly one key, "actionItems", an array of objects — one per actionable task actually ` +
		"mentioned (empty array if there are none actually assigned as follow-up work). Each object has exactly these keys: " +
		`"description" (a concise, self-contained statement of the task), ` +
		`"owner" (the name of the person responsible, exactly as mentioned in the summary — empty string if no owner is named), ` +
		`"dueDate" (an ISO date "YYYY-MM-DD" if a deadline is mentioned or can be reasonably inferred — empty string otherwise), ` +
		`"priority" (one of "low", "medium", "high" — your best judgment of urgency), ` +
		`"confidence" (a number from 0 to 1: how confident you are this is a real, actionable item).` +
		"\n\nSummary:\n" + summaryText
}

func (c *Client) ExtractActionItems(ctx context.Context, summaryText string) ([]entity.ExtractedItem, error) {
	reqBody, err := json.Marshal(generateRequest{
		Model: c.model, Prompt: buildPrompt(summaryText), Stream: false, Format: "json",
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

	var parsed actionItemsJSON
	if err := json.Unmarshal([]byte(gen.Response), &parsed); err != nil {
		return nil, fmt.Errorf("ollama: model did not return valid JSON: %w", err)
	}

	items := make([]entity.ExtractedItem, len(parsed.ActionItems))
	for i, raw := range parsed.ActionItems {
		items[i] = entity.ExtractedItem{
			Description:  raw.Description,
			Type:         entity.TypeAction,
			OwnerRawName: raw.Owner,
			DueDate:      parseDueDate(raw.DueDate),
			Priority:     normalizePriority(raw.Priority),
			Confidence:   raw.Confidence,
		}
	}
	return items, nil
}

func parseDueDate(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}

func normalizePriority(p string) string {
	if entity.ValidPriorities[p] {
		return p
	}
	return entity.PriorityMedium
}
