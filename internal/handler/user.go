package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)


type User struct {
	DB *database.Queries
}

func (u *User) Create (w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	request := struct {
		UserName  string	`json:"UserName"`
		CompanyID uuid.UUID	`json:"CompanyID"`
	}{}

	err := decoder.Decode(&request)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest,"Error decoding request", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second * 10)
	defer cancel()

	dbReq := database.CreateUserParams{
		UserName: request.UserName,
		CompanyID: request.CompanyID,
	}
	user, err := u.DB.CreateUser(ctx, dbReq)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create user:", err)
		return
	}

	data, err := json.Marshal(user)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(data)
}

func (u *User) List (w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	request := struct {
		CompanyID 	uuid.UUID	`json:"CompanyID"`
	}{}

	err := decoder.Decode(&request)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest,"Error decoding request", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second * 10)
	defer cancel()

	users, err := u.DB.GetAllUsersCompany(ctx, request.CompanyID)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create users:", err)
		return
	}

	data, err := json.Marshal(users)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (u *User) GetById (w http.ResponseWriter, r *http.Request) {
	userIDString := chi.URLParam(r,"id")

	userID, err := uuid.Parse(userIDString)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest,"Error missing or invalid user ID:", err)
		return
	}

	decoder := json.NewDecoder(r.Body)
	request := struct {
		CompanyID 	uuid.UUID	`json:"CompanyID"`
	}{}

	err = decoder.Decode(&request)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest,"Error decoding request", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second * 10)
	defer cancel()

	dbReq := database.GetUserParams{
		ID: userID,
		CompanyID: request.CompanyID,
	}
	user, err := u.DB.GetUser(ctx, dbReq)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create user:", err)
		return
	}

	data, err := json.Marshal(user)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (u *User) UpdateById (w http.ResponseWriter, r *http.Request) {
	//no query support for this currently
}

func (u *User) DeleteById (w http.ResponseWriter, r *http.Request) {
	userIDString := chi.URLParam(r,"id")

	userID, err := uuid.Parse(userIDString)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest,"Error missing or invalid user ID:", err)
		return
	}

	decoder := json.NewDecoder(r.Body)
	request := struct {
		CompanyID 	uuid.UUID	`json:"CompanyID"`
	}{}

	err = decoder.Decode(&request)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest,"Error decoding request", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second * 10)
	defer cancel()

	dbReq := database.DeleteUserParams{
		ID: userID,
		CompanyID: request.CompanyID,
	}

	err = u.DB.DeleteUser(ctx, dbReq)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create user:", err)
		return
	}


	w.WriteHeader(http.StatusOK)
}
