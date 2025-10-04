package dbresolver

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/JonMunkholm/RevProject1/internal/ai/credentials"
)

var (
	ErrInvalidReference = errors.New("ai: invalid credential reference")
)

// Cipher is responsible for encrypting/decrypting provider secrets before they are persisted.
type Cipher interface {
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}

// CredentialStore captures the subset of database operations the resolver requires.
type CredentialStore interface {
	FetchCredential(ctx context.Context, companyID, userID uuid.UUID, providerID string) (Record, error)
	TouchCredential(ctx context.Context, companyID, userID uuid.UUID, providerID string) error
	UpsertCredential(ctx context.Context, record Record) error
}

// Record mirrors the ai_provider_credentials table schema.
type Record struct {
	CompanyID        uuid.UUID
	UserID           uuid.UUID
	ProviderID       string
	CredentialCipher []byte
	CredentialHash   []byte
	Metadata         map[string]any
}

// Reference decomposes the credential reference string passed around the system.
type Reference struct {
	CompanyID  uuid.UUID
	UserID     uuid.UUID
	ProviderID string
}

// String encodes the reference into a stable string form.
func (r Reference) String() string {
	return fmt.Sprintf("%s:%s:%s", r.CompanyID, r.UserID, r.ProviderID)
}

// ParseReference converts the string format back into a structured reference.
func ParseReference(ref string) (Reference, error) {
	parts := strings.Split(ref, ":")
	if len(parts) != 3 {
		return Reference{}, ErrInvalidReference
	}
	companyID, err := uuid.Parse(parts[0])
	if err != nil {
		return Reference{}, fmt.Errorf("parse company id: %w", err)
	}
	userID, err := uuid.Parse(parts[1])
	if err != nil {
		return Reference{}, fmt.Errorf("parse user id: %w", err)
	}
	providerID := parts[2]
	if providerID == "" {
		return Reference{}, ErrInvalidReference
	}
	return Reference{CompanyID: companyID, UserID: userID, ProviderID: providerID}, nil
}

// DBResolver implements credentials.Resolver backed by the ai_provider_credentials table.
type DBResolver struct {
	store  CredentialStore
	cipher Cipher
	logger credentials.Logger
}

// New creates a resolver backed by the provided store and cipher. A nil logger falls back to the noop implementation.
func New(store CredentialStore, cipher Cipher, logger credentials.Logger) *DBResolver {
	if logger == nil {
		logger = credentials.NewNoopLogger()
	}
	return &DBResolver{store: store, cipher: cipher, logger: logger}
}

func (r *DBResolver) Resolve(ctx context.Context, reference string) (string, error) {
	ref, err := ParseReference(reference)
	if err != nil {
		return "", err
	}

	rec, err := r.store.FetchCredential(ctx, ref.CompanyID, ref.UserID, ref.ProviderID)
	if err != nil {
		return "", err
	}

	if err := r.store.TouchCredential(ctx, ref.CompanyID, ref.UserID, ref.ProviderID); err != nil {
		r.logger.Warn(ctx, "ai: failed to touch credential", err, map[string]any{"reference": reference})
	}

	plaintext, err := r.cipher.Decrypt(ctx, rec.CredentialCipher)
	if err != nil {
		return "", fmt.Errorf("decrypt credential: %w", err)
	}

	r.logger.Info(ctx, "ai: credential resolved", map[string]any{"reference": reference})
	return string(plaintext), nil
}

func (r *DBResolver) Rotate(ctx context.Context, reference string) error {
	ref, err := ParseReference(reference)
	if err != nil {
		return err
	}

	rec, err := r.store.FetchCredential(ctx, ref.CompanyID, ref.UserID, ref.ProviderID)
	if err != nil {
		return err
	}

	plaintext, err := r.cipher.Decrypt(ctx, rec.CredentialCipher)
	if err != nil {
		return fmt.Errorf("decrypt credential: %w", err)
	}

	ciphertext, err := r.cipher.Encrypt(ctx, plaintext)
	if err != nil {
		return fmt.Errorf("encrypt credential: %w", err)
	}

	rec.CredentialCipher = ciphertext
	rec.CredentialHash = hashSecret(plaintext)
	if err := r.store.UpsertCredential(ctx, rec); err != nil {
		return err
	}

	r.logger.Info(ctx, "ai: credential rotated", map[string]any{"reference": reference})
	return nil
}

func (r *DBResolver) Audit(ctx context.Context, reference string, metadata map[string]any) {
	r.logger.Info(ctx, "ai: credential audit", map[string]any{"reference": reference, "metadata": metadata})
}

func hashSecret(secret []byte) []byte {
	sum := sha256.Sum256(secret)
	return sum[:]
}
