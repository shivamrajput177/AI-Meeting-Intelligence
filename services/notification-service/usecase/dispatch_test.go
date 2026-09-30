package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

func marshalSlack(t *testing.T, text string) []byte {
	t.Helper()
	b, err := json.Marshal(entity.SlackPayload{Text: text})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func marshalEmail(t *testing.T, to, subject, body string) []byte {
	t.Helper()
	b, err := json.Marshal(entity.EmailPayload{To: to, Subject: subject, Body: body})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestDispatchBatch_SlackSuccess(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-1", OrgID: "org-1", Channel: entity.ChannelSlack, Payload: marshalSlack(t, "hello")},
	}}
	slackSender := &fakeSlackSender{}
	emailSender := &fakeEmailSender{}
	pub := &fakePublisher{}
	uc := NewDispatchUseCase(repo, slackSender, emailSender, pub, logger.New("test", logger.LevelError), "https://hooks.slack.example/webhook")

	n, err := uc.DispatchBatch(context.Background(), 20)
	if err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 dispatched, got %d", n)
	}
	if len(slackSender.sent) != 1 || slackSender.sent[0].text != "hello" || slackSender.sent[0].webhookURL != "https://hooks.slack.example/webhook" {
		t.Fatalf("unexpected slack sends: %+v", slackSender.sent)
	}
	if len(repo.sent) != 1 || repo.sent[0] != "row-1" {
		t.Fatalf("expected row-1 marked sent, got %v", repo.sent)
	}
	if len(pub.sent) != 1 || pub.sent[0].OutboxID != "row-1" {
		t.Fatalf("expected notification.sent.v1 published for row-1, got %+v", pub.sent)
	}
}

func TestDispatchBatch_EmailSuccess(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-2", OrgID: "org-1", Channel: entity.ChannelEmail, Payload: marshalEmail(t, "a@b.com", "subj", "body")},
	}}
	slackSender := &fakeSlackSender{}
	emailSender := &fakeEmailSender{}
	pub := &fakePublisher{}
	uc := NewDispatchUseCase(repo, slackSender, emailSender, pub, logger.New("test", logger.LevelError), "")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(emailSender.sent) != 1 || emailSender.sent[0].to != "a@b.com" {
		t.Fatalf("unexpected email sends: %+v", emailSender.sent)
	}
}

func TestDispatchBatch_RetriesOnFailure(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-3", OrgID: "org-1", Channel: entity.ChannelSlack, Attempts: 1, Payload: marshalSlack(t, "hello")},
	}}
	slackSender := &fakeSlackSender{err: errFake}
	emailSender := &fakeEmailSender{}
	pub := &fakePublisher{}
	uc := NewDispatchUseCase(repo, slackSender, emailSender, pub, logger.New("test", logger.LevelError), "url")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(repo.failed) != 1 || repo.failed[0] != "row-3" {
		t.Fatalf("expected row-3 recorded as a failed attempt, got %v", repo.failed)
	}
	if len(pub.failed) != 0 {
		t.Fatalf("expected no notification.failed.v1 yet (attempts below MaxDispatchAttempts), got %+v", pub.failed)
	}
}

func TestDispatchBatch_GivesUpAfterMaxAttempts(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-4", OrgID: "org-1", Channel: entity.ChannelSlack, Attempts: MaxDispatchAttempts - 1, Payload: marshalSlack(t, "hello")},
	}}
	slackSender := &fakeSlackSender{err: errFake}
	emailSender := &fakeEmailSender{}
	pub := &fakePublisher{}
	uc := NewDispatchUseCase(repo, slackSender, emailSender, pub, logger.New("test", logger.LevelError), "url")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(pub.failed) != 1 || pub.failed[0].OutboxID != "row-4" {
		t.Fatalf("expected notification.failed.v1 published for row-4, got %+v", pub.failed)
	}
}

func TestDispatchBatch_UnknownChannel(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-5", OrgID: "org-1", Channel: "carrier-pigeon", Payload: []byte("{}")},
	}}
	uc := NewDispatchUseCase(repo, &fakeSlackSender{}, &fakeEmailSender{}, &fakePublisher{}, logger.New("test", logger.LevelError), "")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(repo.failed) != 1 {
		t.Fatalf("expected the unknown-channel row recorded as a failed attempt, got %v", repo.failed)
	}
}
