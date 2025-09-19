package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)


type Company struct {
	DB *database.Queries
}

func (c *Company) Create (w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	request := struct {
		CompanyName	string `json:"CompanyName"`
		UserName	string	`json:"UserName"`
	}{}

	err := decoder.Decode(&request)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest,"Error decoding request", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second * 10)
	defer cancel()

	company, err := c.DB.CreateCompany(ctx, request.CompanyName)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create Company:", err)
		return
	}

	firstUser := database.CreateUserParams{
		UserName: request.UserName,
		CompanyID: company.ID,
	}

	user, err := c.DB.CreateUser(ctx, firstUser)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create user:", err)
		return
	}

	res := struct{
		Company 	database.Company  `json:"company"`
		User		database.User	  `json:"user"`
	}{
		Company: company,
		User: user,
	}

	data, err := json.Marshal(res)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(data)
}

func (c *Company) List (w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second * 10)
	defer cancel()

	companies, err := c.DB.GetAllCompanies(ctx)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve companies list:", err)
		return
	}

	res, err := json.Marshal(companies)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to marshal response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func (c *Company) GetById (w http.ResponseWriter, r *http.Request) {
	companyIDString := chi.URLParam(r,"id")
	fmt.Println(companyIDString)
	companyID, err := uuid.Parse(companyIDString)
	if err != nil {
		RespondWithError(w, http.StatusNotFound, "Failed to parse id to UUID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second * 10)
	defer cancel()

	company, err := c.DB.GetCompany(ctx, companyID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to get company by ID:", err)
		return
	}

	res, err := json.Marshal(company)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to marshal response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func (c *Company) UpdateById (w http.ResponseWriter, r *http.Request) {
	//no query support for this currently
}

func (c *Company) DeleteById (w http.ResponseWriter, r *http.Request) {
	companyIDString := chi.URLParam(r,"id")

	companyID, err := uuid.Parse(companyIDString)
	if err != nil {
		RespondWithError(w, http.StatusNotFound, "Failed to parse id to UUID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second * 10)
	defer cancel()

	err = c.DB.DeleteCompany(ctx, companyID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to get company by ID:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
