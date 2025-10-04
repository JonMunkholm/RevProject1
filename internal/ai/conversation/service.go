package conversation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	clientpkg "github.com/JonMunkholm/RevProject1/internal/ai/client"
)

// Store defines the database operations required by the conversation service.
type Store interface {
	CreateSession(ctx context.Context, params CreateSessionParams) (Session, error)
	UpdateSessionTitle(ctx context.Context, id uuid.UUID, companyID uuid.UUID, title string) error
	ListSessions(ctx context.Context, companyID uuid.UUID, limit, offset int32) ([]Session, error)
	DeleteSession(ctx context.Context, id uuid.UUID, companyID uuid.UUID) error
	InsertMessage(ctx context.Context, params CreateMessageParams) (Message, error)
	ListMessages(ctx context.Context, sessionID uuid.UUID) ([]Message, error)
	DeleteMessages(ctx context.Context, sessionID uuid.UUID) error
}

// Session represents a persisted conversation container.
type Session struct {
	ID         uuid.UUID
	CompanyID  uuid.UUID
	UserID     uuid.UUID
	ProviderID string
	Title      string
	Metadata   map[string]any
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Message represents a single exchange within a session.
type Message struct {
	ID        uuid.UUID
	SessionID uuid.UUID
	Role      string
	Content   string
	Metadata  map[string]any
	CreatedAt time.Time
}

type CreateSessionParams struct {
	CompanyID  uuid.UUID
	UserID     uuid.UUID
	ProviderID string
	Title      string
	Metadata   map[string]any
}

type CreateMessageParams struct {
	SessionID uuid.UUID
	Role      string
	Content   string
	Metadata  map[string]any
}

// Service coordinates conversation persistence.
type Service struct {
	store  Store
	logger clientpkg.Logger
}

// New returns a conversation service backed by the supplied store and logger.
func New(store Store, logger clientpkg.Logger) *Service {
	if logger == nil {
		logger = clientpkg.NewNoopLogger()
	}
	return &Service{store: store, logger: logger}
}

func (s *Service) StartSession(ctx context.Context, params CreateSessionParams) (Session, error) {
	session, err := s.store.CreateSession(ctx, params)
	if err != nil {
		return Session{}, err
	}
	s.logger.Info(ctx, "ai: conversation session created", map[string]any{"session_id": session.ID, "company_id": session.CompanyID})
	return session, nil
}

func (s *Service) RenameSession(ctx context.Context, companyID, sessionID uuid.UUID, title string) error {
	if title == "" {
		return errors.New("ai: session title cannot be empty")
	}
	if err := s.store.UpdateSessionTitle(ctx, sessionID, companyID, title); err != nil {
		return err
	}
	s.logger.Info(ctx, "ai: conversation session renamed", map[string]any{"session_id": sessionID, "company_id": companyID})
	return nil
}

func (s *Service) RemoveSession(ctx context.Context, companyID, sessionID uuid.UUID) error {
	if err := s.store.DeleteMessages(ctx, sessionID); err != nil {
		return err
	}
	if err := s.store.DeleteSession(ctx, sessionID, companyID); err != nil {
		return err
	}
	s.logger.Info(ctx, "ai: conversation session deleted", map[string]any{"session_id": sessionID, "company_id": companyID})
	return nil
}

func (s *Service) AppendMessage(ctx context.Context, params CreateMessageParams) (Message, error) {
	msg, err := s.store.InsertMessage(ctx, params)
	if err != nil {
		return Message{}, err
	}
	s.logger.Info(ctx, "ai: conversation message appended", map[string]any{"session_id": params.SessionID, "role": params.Role})
	return msg, nil
}

func (s *Service) ListCompanySessions(ctx context.Context, companyID uuid.UUID, limit, offset int32) ([]Session, error) {
	return s.store.ListSessions(ctx, companyID, limit, offset)
}

func (s *Service) ListSessionMessages(ctx context.Context, sessionID uuid.UUID) ([]Message, error) {
	return s.store.ListMessages(ctx, sessionID)
}
