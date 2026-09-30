package usecase

import (
	"context"
	"errors"
	"sync"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/meetings"
)

// fakeRepository, fakeMeetingsClient, fakeUsersClient, fakeSlackSender,
// fakeEmailSender, and fakePublisher are in-memory stand-ins for the real
// Postgres-/HTTP-backed implementations — this package's own tests never
// touch a live Postgres or another service over HTTP.

type enqueuedRow struct {
	orgID, channel string
	payload        []byte
}

type fakeRepository struct {
	mu       sync.Mutex
	enqueued []enqueuedRow

	claimRows []entity.OutboxRow
	claimErr  error

	sent    []string
	failed  []string
	markErr error
}

func (f *fakeRepository) Enqueue(_ context.Context, orgID, channel string, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enqueued = append(f.enqueued, enqueuedRow{orgID, channel, payload})
	return nil
}

func (f *fakeRepository) ClaimBatch(context.Context, int) ([]entity.OutboxRow, error) {
	return f.claimRows, f.claimErr
}

func (f *fakeRepository) MarkSent(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markErr != nil {
		return f.markErr
	}
	f.sent = append(f.sent, id)
	return nil
}

func (f *fakeRepository) MarkAttemptFailed(_ context.Context, id string, _ int, _ string, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.markErr != nil {
		return f.markErr
	}
	f.failed = append(f.failed, id)
	return nil
}

type fakeMeetingsClient struct {
	meeting *meetings.Meeting
	err     error
}

func (f *fakeMeetingsClient) GetMeeting(context.Context, string, string) (*meetings.Meeting, error) {
	return f.meeting, f.err
}

type fakeUsersClient struct {
	email string
	err   error
}

func (f *fakeUsersClient) GetEmail(context.Context, string, string) (string, error) {
	return f.email, f.err
}

type sentMessage struct{ to, subject, body string }
type sentSlack struct{ webhookURL, text string }

type fakeSlackSender struct {
	mu   sync.Mutex
	sent []sentSlack
	err  error
}

func (f *fakeSlackSender) Send(_ context.Context, webhookURL, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, sentSlack{webhookURL, text})
	return nil
}

type fakeEmailSender struct {
	mu   sync.Mutex
	sent []sentMessage
	err  error
}

func (f *fakeEmailSender) Send(_ context.Context, to, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, sentMessage{to, subject, body})
	return nil
}

type fakePublisher struct {
	mu     sync.Mutex
	sent   []entity.NotificationSentEvent
	failed []entity.NotificationFailedEvent
}

func (f *fakePublisher) PublishNotificationSent(_ context.Context, event entity.NotificationSentEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, event)
	return nil
}

func (f *fakePublisher) PublishNotificationFailed(_ context.Context, event entity.NotificationFailedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = append(f.failed, event)
	return nil
}

var errFake = errors.New("fake error")
