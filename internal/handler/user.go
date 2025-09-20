package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type User struct {
	DB *database.Queries
}

type createUserRequest struct {
	UserName  string    `json:"UserName"`
	CompanyID uuid.UUID `json:"CompanyID"`
}

type companyUsersRequest struct {
	CompanyID uuid.UUID `json:"CompanyID"`
}

func (u *User) Create(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() createUserRequest { return createUserRequest{} },
		func(req createUserRequest) (database.CreateUserParams, error) {
			companyID, err := companyIDFromRequest(req.CompanyID)
			if err != nil {
				return database.CreateUserParams{}, err
			}

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

func (u *User) List(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() companyUsersRequest { return companyUsersRequest{} },
		func(req companyUsersRequest) (uuid.UUID, error) {
			return companyIDFromRequest(req.CompanyID)
		},
		func(ctx context.Context, param uuid.UUID) ([]database.User, error) {
			return u.DB.GetAllUsersCompany(ctx, param)
		},
		http.StatusOK,
	)

}

func (u *User) GetById(w http.ResponseWriter, r *http.Request) {

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
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
		func() companyUsersRequest { return companyUsersRequest{} },
		func(req companyUsersRequest) (database.GetUserParams, error) {
			companyID, err := companyIDFromRequest(req.CompanyID)
			if err != nil {
				return database.GetUserParams{}, err
			}

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

func (u *User) UpdateById(w http.ResponseWriter, r *http.Request) {
	//no query support for this currently
}

func (u *User) DeleteById(w http.ResponseWriter, r *http.Request) {

	userID, err := uuid.Parse(chi.URLParam(r, "id"))
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
		func() companyUsersRequest { return companyUsersRequest{} },
		func(req companyUsersRequest) (database.DeleteUserParams, error) {
			companyID, err := companyIDFromRequest(req.CompanyID)
			if err != nil {
				return database.DeleteUserParams{}, err
			}

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
