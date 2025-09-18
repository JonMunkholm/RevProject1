package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	helper "github.com/JonMunkholm/RevProject1/internal"
	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)


type Company struct {
	DB *database.Queries
}

func (u *Company) Create (w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	request := struct {
		CompanyName	string `json:"CompanyName"`
		UserName	string	`json:"UserName"`
	}{}

	err := decoder.Decode(&request)

	if err != nil {
		helper.RespondWithError(w, http.StatusInternalServerError,"Error decoding request", err)
		return
	}

	timeout, cancel := context.WithTimeout(context.Background(), time.Second * 10)
	defer cancel()

	company, err := u.DB.CreateCompany(timeout, request.CompanyName)
	if err != nil {
		helper.RespondWithError(w, http.StatusInternalServerError, "Failed to create Company:", err)
		return
	}

	firstUser := database.CreateUserParams{
		UserName: request.UserName,
		CompanyID: company.ID,
	}

	user, err := u.DB.CreateUser(timeout, firstUser)
	if err != nil {
		helper.RespondWithError(w, http.StatusInternalServerError, "Failed to create user:", err)
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
		helper.RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(data)
}

func (u *Company) List (w http.ResponseWriter, r *http.Request) {
	timeout, cancel := context.WithTimeout(context.Background(), time.Second * 10)
	defer cancel()

	companies, err := u.DB.GetAllCompanies(timeout)
	if err != nil {
		helper.RespondWithError(w, http.StatusInternalServerError, "Failed to retrieve companies list:", err)
		return
	}

	res, err := json.Marshal(companies)
	if err != nil {
		helper.RespondWithError(w, http.StatusInternalServerError, "Failed to marshal response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func (u *Company) GetById (w http.ResponseWriter, r *http.Request) {
	companyIDString := chi.URLParam(r,"id")
	fmt.Println(companyIDString)
	companyID, err := uuid.Parse(companyIDString)
	if err != nil {
		helper.RespondWithError(w, http.StatusInternalServerError, "Failed to parse id to UUID:", err)
		return
	}

	timeout, cancel := context.WithTimeout(context.Background(), time.Second * 10)
	defer cancel()

	company, err := u.DB.GetCompany(timeout, companyID)
	if err != nil {
		helper.RespondWithError(w, http.StatusInternalServerError, "Failed to get company by ID:", err)
		return
	}

	res, err := json.Marshal(company)
	if err != nil {
		helper.RespondWithError(w, http.StatusInternalServerError, "Failed to marshal response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func (u *Company) UpdateById (w http.ResponseWriter, r *http.Request) {
	//no query support for this currently
}

func (u *Company) DeleteById (w http.ResponseWriter, r *http.Request) {
	companyIDString := chi.URLParam(r,"id")

	companyID, err := uuid.Parse(companyIDString)
	if err != nil {
		helper.RespondWithError(w, http.StatusInternalServerError, "Failed to parse id to UUID:", err)
		return
	}

	timeout, cancel := context.WithTimeout(context.Background(), time.Second * 10)
	defer cancel()

	err = u.DB.DeleteCompany(timeout, companyID)
	if err != nil {
		helper.RespondWithError(w, http.StatusInternalServerError, "Failed to get company by ID:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
