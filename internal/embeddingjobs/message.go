package embeddingjobs

import (
	"time"

	"github.com/google/uuid"
)

// Message represents the payload sent to the embedding job queue.
type Message struct {
	JobID           uuid.UUID `json:"job_id"`
	ParagraphID     uuid.UUID `json:"paragraph_id"`
	SourceHash      string    `json:"source_hash"`
	Model           string    `json:"model"`
	Priority        string    `json:"priority"`
	MetadataVersion string    `json:"metadata_version"`
	CreatedAt       time.Time `json:"created_at"`
}
