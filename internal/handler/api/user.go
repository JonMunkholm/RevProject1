package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type User struct {
	DB *database.Queries
}

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type updateUserRequest struct {
	Email    string `json:"email"`
	IsActive bool   `json:"IsActive"`
}

// userResponse contains the user fields that are safe to expose via the API.
type userResponse struct {
	ID        uuid.UUID `json:"ID"`
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`
	CompanyID uuid.UUID `json:"CompanyID"`
	Email     string    `json:"Email"`
	IsActive  bool      `json:"IsActive"`
}


func (u *User) Create(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() createUserRequest { return createUserRequest{} },
		func(req createUserRequest) (database.CreateUserParams, error) {
			email := strings.TrimSpace(req.Email)
			if email == "" {
				return database.CreateUserParams{}, errors.New("email is required")
			}

			if !isValidEmail(email) {
				return database.CreateUserParams{}, errors.New("invalid email format")
			}

			if len(req.Password) < 8 {
				return database.CreateUserParams{}, errors.New("password must be at least 8 characters")
			}

			hashed, err := auth.HashPassword(req.Password)
			if err != nil {
				return database.CreateUserParams{}, err
			}

			return database.CreateUserParams{
				CompanyID:    companyID,
				Email:        email,
				PasswordHash: hashed,
			}, nil
		},
		func(ctx context.Context, params database.CreateUserParams) (userResponse, error) {
			created, err := u.DB.CreateUser(ctx, params)
			if err != nil {
				return userResponse{}, err
			}
			return newUserResponse(created), nil
		},
		http.StatusCreated,
	)
}

func (u *User) ListAll(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	users, err := u.DB.GetAllUsers(ctx)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve users list:", err)
		return
	}

	res, err := json.Marshal(newUserList(users))
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to marshal response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)

}

func (u *User) List(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (uuid.UUID, error) {
			return companyID, nil
		},
		func(ctx context.Context, param uuid.UUID) ([]userResponse, error) {
			users, err := u.DB.GetAllUsersCompany(ctx, param)
			if err != nil {
				return nil, err
			}
			return newUserList(users), nil
		},
		http.StatusOK,
	)

}

func (u *User) GetById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid user ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetUserParams, error) {
			return database.GetUserParams{
				ID:        userID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetUserParams) (userResponse, error) {
			user, err := u.DB.GetUser(ctx, param)
			if err != nil {
				return userResponse{}, err
			}
			return newUserResponse(user), nil
		},
		http.StatusOK,
	)

}

func (u *User) GetByEmail(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	email := strings.TrimSpace(chi.URLParam(r, "email"))

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetUserByEmailParams, error) {
			return database.GetUserByEmailParams{
				CompanyID: companyID,
				Email:     email,
			}, nil
		},
		func(ctx context.Context, param database.GetUserByEmailParams) (userResponse, error) {
			user, err := u.DB.GetUserByEmail(ctx, param)
			if err != nil {
				return userResponse{}, err
			}
			return newUserResponse(user), nil
		},
		http.StatusOK,
	)
}

func (u *User) UpdateById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid user ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() updateUserRequest { return updateUserRequest{} },
		func(req updateUserRequest) (database.UpdateUserParams, error) {
			email := strings.TrimSpace(req.Email)
			if email == "" {
				return database.UpdateUserParams{}, errors.New("email is required")
			}

			if !isValidEmail(email) {
				return database.UpdateUserParams{}, errors.New("invalid email format")
			}

			return database.UpdateUserParams{
				Email:     email,
				IsActive:  req.IsActive,
				ID:        userID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.UpdateUserParams) (userResponse, error) {
			updated, err := u.DB.UpdateUser(ctx, param)
			if err != nil {
				return userResponse{}, err
			}
			return newUserResponse(updated), nil
		},
		http.StatusOK,
	)

}

func (u *User) DeleteById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid user ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.DeleteUserParams, error) {
			return database.DeleteUserParams{
				ID:        userID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.DeleteUserParams) (struct{}, error) {
			return struct{}{}, u.DB.DeleteUser(ctx, param)
		},
		http.StatusOK,
	)

}

func isValidEmail(email string) bool {
	at := strings.IndexRune(email, '@')
	return at > 0 && at < len(email)-1 && strings.IndexRune(email[at+1:], '.') >= 0
}

func (u *User) ResetTable(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	err := u.DB.ResetUsers(ctx)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to reset users table", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, struct{}{})
}

func (u *User) GetActive(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (uuid.UUID, error) {
			return companyID, nil
		},
		func(ctx context.Context, param uuid.UUID) ([]userResponse, error) {
			users, err := u.DB.GetActiveUsersCompany(ctx, param)
			if err != nil {
				return nil, err
			}
			return newUserList(users), nil
		},
		http.StatusOK,
	)

}

func (u *User) SetActive(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	userID, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid user ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() setActiveRequest { return setActiveRequest{} },
		func(req setActiveRequest) (database.SetUserActiveStatusParams, error) {
			return database.SetUserActiveStatusParams{
				IsActive:  req.IsActive,
				ID:        userID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.SetUserActiveStatusParams) (struct{}, error) {
			return struct{}{}, u.DB.SetUserActiveStatus(ctx, param)
		},
		http.StatusOK,
	)

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
