package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/storage"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type ConfirmUploadUseCase struct {
	repo           repository.Repository
	storage        storage.ObjectStorage
	publisher      events.Publisher
	log            *logger.Logger
	maxUploadBytes int64
}

// maxUploadBytes is the Phase 7 public-demo clip-length guard (see
// docs/architecture/deployment-demo-strategy.md §2's "≤2 min clips" rule):
// 0 means unlimited (every non-demo deployment — Kind, docker-compose dev,
// the config's own dev-safe default), a positive value rejects any upload
// over that many bytes. It's a size cap standing in for a true wall-clock
// duration cap — this service never decodes the audio, so it can't measure
// seconds directly, only bytes; a generous size still comfortably bounds a
// short clip across the codecs a browser's MediaRecorder actually produces.
func NewConfirmUploadUseCase(repo repository.Repository, storage storage.ObjectStorage, publisher events.Publisher, log *logger.Logger, maxUploadBytes int64) *ConfirmUploadUseCase {
	return &ConfirmUploadUseCase{repo: repo, storage: storage, publisher: publisher, log: log, maxUploadBytes: maxUploadBytes}
}

// ConfirmUpload verifies the object actually landed in MinIO before treating
// the upload as real — a client that calls this without ever PUTting the
// file would otherwise leave a meeting row pointing at nothing. Once
// confirmed, it publishes meeting.uploaded.v1 to kick off the Phase 2
// pipeline (see docs/architecture/kafka-topics.md's event-flow diagram).
//
// This is also where a trace actually begins (see shared/kafkax's doc
// comment on HeaderTraceparent): every later Kafka event this upload's
// processing produces, across every service, is a ChildTraceparent of the
// id minted here, so one trace_id greps a single meeting's whole pipeline
// run out of every service's logs.
func (uc *ConfirmUploadUseCase) ConfirmUpload(ctx context.Context, orgID, meetingID string) (*entity.Meeting, error) {
	meeting, err := uc.repo.GetByID(ctx, orgID, meetingID)
	if err != nil {
		return nil, err
	}
	size, err := uc.storage.Stat(ctx, meeting.RecordingObjectKey)
	if err != nil {
		return nil, apperr.BadRequest("recording not found — did the upload complete?")
	}
	if uc.maxUploadBytes > 0 && size > uc.maxUploadBytes {
		// Best-effort cleanup: an oversized object sitting in MinIO forever
		// isn't harmful (the meeting row never leaves "uploaded" either way,
		// same as the publish-failure path below), so a Delete error here is
		// logged, not fatal to rejecting the request.
		if delErr := uc.storage.Delete(ctx, meeting.RecordingObjectKey); delErr != nil {
			uc.log.Error("delete oversized demo upload", "meeting_id", meetingID, "err", delErr)
		}
		return nil, apperr.BadRequest("recording exceeds this demo's clip-length limit — try a shorter one")
	}
	if err := uc.repo.Touch(ctx, orgID, meetingID); err != nil {
		return nil, apperr.Internal("update meeting").Wrap(err)
	}
	updated, err := uc.repo.GetByID(ctx, orgID, meetingID)
	if err != nil {
		return nil, err
	}

	traceparent := kafkax.NewTraceparent()
	traceCtx := reqctx.WithTraceparent(ctx, traceparent)

	// Best-effort publish: Kafka being down shouldn't fail a request that
	// already durably confirmed the upload in Postgres+MinIO. There's no
	// transactional outbox yet (Phase 4) to make this delivery guaranteed
	// — the same kind of known gap as Signup's cross-service
	// orchestration in auth-service's SignupUseCase — so a dropped event
	// here means the meeting sits in "uploaded" until a future
	// retry/reconciliation mechanism exists. Logged loudly rather than
	// silently swallowed, since it's a real gap, not a non-issue.
	if err := uc.publisher.PublishMeetingUploaded(traceCtx, entity.MeetingUploadedEvent{
		MeetingID: updated.ID, OrgID: updated.OrgID, Title: updated.Title,
		SourceType: updated.SourceType, RecordingObjectKey: updated.RecordingObjectKey,
	}); err != nil {
		uc.log.Error("publish meeting.uploaded.v1 failed", "meeting_id", updated.ID, "trace_id", kafkax.TraceIDOf(traceparent), "err", err)
	}
	return updated, nil
}
