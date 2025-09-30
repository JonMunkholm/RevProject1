package handler

import (
	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/google/uuid"
	"time"
)

// userResponse contains the user fields that are safe to expose via the API.
type userResponse struct {
	ID        uuid.UUID `json:"ID"`
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`
	CompanyID uuid.UUID `json:"CompanyID"`
	Email     string    `json:"Email"`
	IsActive  bool      `json:"IsActive"`
}

func newUserResponse(u database.User) userResponse {
	return userResponse{
		ID:        u.ID,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		CompanyID: u.CompanyID,
		Email:     u.Email,
		IsActive:  u.IsActive,
	}
}

func newUserList(users []database.User) []userResponse {
	sanitized := make([]userResponse, 0, len(users))
	for _, u := range users {
		sanitized = append(sanitized, newUserResponse(u))
	}
	return sanitized
}
