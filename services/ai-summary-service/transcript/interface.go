// Package transcript defines the Client interface usecase depends on;
// transcript/http, right below this package in the same tree, implements
// it against Transcription Service's internal REST API — see that
// package's doc comment. Keeping the interface here instead of off in
// some unrelated package is just where it belongs — its one real
// implementation lives one directory down.
package transcript

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
)

type Client interface {
	GetTranscript(ctx context.Context, orgID, meetingID string) (*entity.Transcript, error)
}
