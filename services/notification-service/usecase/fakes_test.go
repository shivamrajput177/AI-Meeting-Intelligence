package usecase

import (
	"context"
	"errors"
	"sync"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/meetings"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/ticketprovider"
)

// fakeRepository, fakeJiraRepository, fakeMeetingsClient, fakeUsersClient,
// fakeActionItemsClient, fakeSlackSender, fakeEmailSender,
// fakeTicketProvider, and fakePublisher are in-memory stand-ins for the
// real Postgres-/HTTP-backed implementations — this package's own tests
// never touch a live Postgres or another service over HTTP.

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

type mockIssue struct{ orgID, actionItemID, title string }
type transitionCall struct{ orgID, issueKey, status string }
type jiraLinkCall struct{ orgID, actionItemID, provider, issueKey, url string }

type fakeJiraRepository struct {
	mu sync.Mutex

	created      []mockIssue
	nextIssueKey string
	createErr    error

	transitions      []transitionCall
	transitionResult string
	transitionErr    error

	board    []entity.MockJiraIssue
	boardErr error

	links []jiraLinkCall
}

func (f *fakeJiraRepository) CreateMockIssue(_ context.Context, orgID, actionItemID, title string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return "", f.createErr
	}
	f.created = append(f.created, mockIssue{orgID, actionItemID, title})
	return f.nextIssueKey, nil
}

func (f *fakeJiraRepository) TransitionMockIssue(_ context.Context, orgID, issueKey, newStatus string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.transitionErr != nil {
		return "", f.transitionErr
	}
	f.transitions = append(f.transitions, transitionCall{orgID, issueKey, newStatus})
	return f.transitionResult, nil
}

func (f *fakeJiraRepository) ListMockBoard(context.Context, string) ([]entity.MockJiraIssue, error) {
	return f.board, f.boardErr
}

func (f *fakeJiraRepository) UpsertJiraLink(_ context.Context, orgID, actionItemID, provider, issueKey, url string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.links = append(f.links, jiraLinkCall{orgID, actionItemID, provider, issueKey, url})
	return nil
}

type updateActionItemCall struct {
	orgID, actionItemID string
	status, jiraKey     *string
}

type fakeActionItemsClient struct {
	mu      sync.Mutex
	updates []updateActionItemCall
	err     error
}

func (f *fakeActionItemsClient) UpdateActionItem(_ context.Context, orgID, actionItemID string, status, jiraIssueKey *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.updates = append(f.updates, updateActionItemCall{orgID, actionItemID, status, jiraIssueKey})
	return nil
}

type fakeTicketProvider struct {
	ref ticketprovider.TicketRef
	err error
}

func (f *fakeTicketProvider) CreateTicket(context.Context, string, string, string) (ticketprovider.TicketRef, error) {
	return f.ref, f.err
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
	mu           sync.Mutex
	sent         []entity.NotificationSentEvent
	failed       []entity.NotificationFailedEvent
	remindersDue []entity.ActionItemReminderDueEvent
	reminderErr  error
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

func (f *fakePublisher) PublishActionItemReminderDue(_ context.Context, event entity.ActionItemReminderDueEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reminderErr != nil {
		return f.reminderErr
	}
	f.remindersDue = append(f.remindersDue, event)
	return nil
}

// fakeReminderRepository is an in-memory stand-in for
// *postgres.ReminderRepository. lockAcquired defaults to true (the common
// case in tests that don't care about leader election); set it false to
// simulate another replica already holding the lock this tick.
type fakeReminderRepository struct {
	mu sync.Mutex

	lockAcquired bool
	lockCalled   bool

	dueRows  []entity.DueReminder
	claimErr error

	sent []string
}

func newFakeReminderRepository() *fakeReminderRepository {
	return &fakeReminderRepository{lockAcquired: true}
}

func (f *fakeReminderRepository) WithLeaderLock(ctx context.Context, fn func(ctx context.Context) error) error {
	f.mu.Lock()
	f.lockCalled = true
	acquired := f.lockAcquired
	f.mu.Unlock()
	if !acquired {
		return nil
	}
	return fn(ctx)
}

func (f *fakeReminderRepository) ClaimDueReminders(context.Context, int) ([]entity.DueReminder, error) {
	return f.dueRows, f.claimErr
}

func (f *fakeReminderRepository) MarkReminderSent(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, id)
	return nil
}

var errFake = errors.New("fake error")
