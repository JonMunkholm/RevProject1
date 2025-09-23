package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type User struct {
	DB *database.Queries
}

type createUserRequest struct {
	UserName string `json:"UserName"`
}

type updateUserRequest struct {
	UserName string `json:"UserName"`
	IsActive bool   `json:"IsActive"`
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
			return database.CreateUserParams{
				UserName:  req.UserName,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, params database.CreateUserParams) (database.User, error) {
			return u.DB.CreateUser(ctx, params)
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

	res, err := json.Marshal(users)
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
		func(ctx context.Context, param uuid.UUID) ([]database.User, error) {
			return u.DB.GetAllUsersCompany(ctx, param)
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
		func(ctx context.Context, param database.GetUserParams) (database.User, error) {
			return u.DB.GetUser(ctx, param)
		},
		http.StatusOK,
	)

}

func (u *User) GetByName(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	userName := strings.TrimSpace(chi.URLParam(r, "name"))

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetUserByNameParams, error) {
			return database.GetUserByNameParams{
				CompanyID: companyID,
				UserName:  userName,
			}, nil
		},
		func(ctx context.Context, param database.GetUserByNameParams) (database.User, error) {
			return u.DB.GetUserByName(ctx, param)
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
			return database.UpdateUserParams{
				UserName:  req.UserName,
				IsActive:  req.IsActive,
				ID:        userID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.UpdateUserParams) (database.User, error) {
			return u.DB.UpdateUser(ctx, param)
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
		func(ctx context.Context, param uuid.UUID) ([]database.User, error) {
			return u.DB.GetActiveUsersCompany(ctx, param)
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
