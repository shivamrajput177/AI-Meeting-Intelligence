// Package asr defines the Transcriber interface usecase depends on;
// asr/whispercpp, right below this package in the same tree, implements
// it against a real whisper.cpp server — see that package's doc comment.
// Keeping the interface here instead of off in some unrelated package is
// just where it belongs — its one real implementation lives one
// directory down.
package asr

import (
	"context"
	"io"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/entity"
)

// Transcriber runs speech-to-text against an audio stream, named for the
// filename (so implementations that shell out or care about extension
// have it, even when reading from an io.Reader rather than a real file).
type Transcriber interface {
	Transcribe(ctx context.Context, audio io.Reader, filename string) (*entity.TranscriptionResult, error)
}
