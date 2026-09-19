// Package whispercpp implements asr.Transcriber against a real
// whisper.cpp server's /inference REST endpoint (whisper.cpp's
// examples/server, run as its own container — see
// deployments/docker-compose.yaml) — see
// docs/architecture/microservices.md §6: "extract audio with ffmpeg ->
// POST to whisper.cpp /inference". whisper.cpp's server deliberately
// mirrors OpenAI's /v1/audio/transcriptions request/response shape for
// drop-in compatibility, which is the contract this client codes
// against: multipart POST with the audio under "file" and
// response_format=verbose_json, decoding a {text, language,
// segments:[{start, end, text}]} JSON body (start/end in float seconds).
//
// Phase 2 doesn't do the ffmpeg extraction step — source recordings are
// assumed already in a format whisper.cpp accepts (wav/mp3); that's a
// gap to close the moment a container format whisper.cpp can't read
// directly shows up in practice, not something this client silently
// works around.
package whispercpp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/entity"
)

type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a client against a whisper.cpp server already running with
// the desired model loaded (whisper.cpp loads one model per server
// process — model selection is a server-startup concern, not a
// per-request field this client sends).
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		// Whisper inference on CPU for a real meeting recording can
		// legitimately take minutes — see
		// docs/architecture/microservices.md §6's scaling note on this
		// being a long-running, CPU-bound job.
		http: &http.Client{Timeout: 30 * time.Minute},
	}
}

type whisperSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

type whisperResponse struct {
	Text     string           `json:"text"`
	Language string           `json:"language"`
	Segments []whisperSegment `json:"segments"`
}

func (c *Client) Transcribe(ctx context.Context, audio io.Reader, filename string) (*entity.TranscriptionResult, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("whispercpp: create form file: %w", err)
	}
	if _, err := io.Copy(part, audio); err != nil {
		return nil, fmt.Errorf("whispercpp: copy audio into request: %w", err)
	}
	if err := w.WriteField("response_format", "verbose_json"); err != nil {
		return nil, fmt.Errorf("whispercpp: write response_format field: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("whispercpp: close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/inference", &body)
	if err != nil {
		return nil, fmt.Errorf("whispercpp: build request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whispercpp: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("whispercpp: server returned %d: %s", resp.StatusCode, string(b))
	}

	var wr whisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&wr); err != nil {
		return nil, fmt.Errorf("whispercpp: decode response: %w", err)
	}

	result := &entity.TranscriptionResult{Language: wr.Language, Text: wr.Text}
	for _, s := range wr.Segments {
		result.Segments = append(result.Segments, entity.TranscribedSegment{
			StartMS: int(s.Start * 1000), EndMS: int(s.End * 1000), Text: s.Text,
		})
	}
	return result, nil
}
