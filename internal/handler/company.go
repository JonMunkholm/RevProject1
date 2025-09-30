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

type Company struct {
	DB *database.Queries
}

type createCompanyRequest struct {
	CompanyName string `json:"CompanyName"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type createCompanyArgs struct {
	CompanyName string
	Email       string
	Password    string
}

type setActiveRequest struct {
	IsActive bool `json:"IsActive"`
}

type updateCompanyRequest struct {
	CompanyName string `json:"CompanyName"`
	IsActive    bool   `json:"IsActive"`
}

type createCompanyResponse struct {
	Company database.Company `json:"company"`
	User    userResponse     `json:"user"`
}

func (c *Company) Create(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() createCompanyRequest { return createCompanyRequest{} },
		func(req createCompanyRequest) (createCompanyArgs, error) {
			companyName := strings.TrimSpace(req.CompanyName)
			email := strings.TrimSpace(req.Email)

			switch {
			case companyName == "":
				return createCompanyArgs{}, errors.New("CompanyName is required")
			case email == "":
				return createCompanyArgs{}, errors.New("email is required")
			case len(req.Password) < 8:
				return createCompanyArgs{}, errors.New("password must be at least 8 characters")
			}

			if !isValidEmail(email) {
				return createCompanyArgs{}, errors.New("invalid email format")
			}

			return createCompanyArgs{
				CompanyName: companyName,
				Email:       email,
				Password:    req.Password,
			}, nil
		},
		func(ctx context.Context, params createCompanyArgs) (createCompanyResponse, error) {
			company, err := c.DB.CreateCompany(ctx, params.CompanyName)
			if err != nil {
				return createCompanyResponse{}, err
			}

			hashed, err := auth.HashPassword(params.Password)
			if err != nil {
				return createCompanyResponse{}, err
			}

			user, err := c.DB.CreateUser(ctx, database.CreateUserParams{
				CompanyID:    company.ID,
				Email:        params.Email,
				PasswordHash: hashed,
			})
			if err != nil {
				return createCompanyResponse{}, err
			}

			return createCompanyResponse{
				Company: company,
				User:    newUserResponse(user),
			}, nil
		},
		http.StatusCreated,
	)
}

func (c *Company) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
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

func (c *Company) GetById(w http.ResponseWriter, r *http.Request) {
	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusNotFound, "Failed to parse id to UUID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	company, err := c.DB.GetCompany(ctx, companyID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to get company by ID:", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, company)
}

func (c *Company) GetByName(w http.ResponseWriter, r *http.Request) {

	companyName := strings.TrimSpace(chi.URLParam(r, "name"))

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	company, err := c.DB.GetCompanyByName(ctx, companyName)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to get company by ID:", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, company)
}

func (c *Company) GetActive(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	companies, err := c.DB.GetActiveCompanies(ctx)
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

func (c *Company) SetActive(w http.ResponseWriter, r *http.Request) {
	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusNotFound, "Failed to parse id to UUID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() setActiveRequest { return setActiveRequest{} },
		func(req setActiveRequest) (database.SetCompanyActiveStatusParams, error) {
			return database.SetCompanyActiveStatusParams{
				IsActive: req.IsActive,
				ID:       companyID,
			}, nil
		},
		func(ctx context.Context, params database.SetCompanyActiveStatusParams) (struct{}, error) {
			return struct{}{}, c.DB.SetCompanyActiveStatus(ctx, params)
		},
		http.StatusOK,
	)
}

func (c *Company) UpdateById(w http.ResponseWriter, r *http.Request) {
	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusNotFound, "Failed to parse id to UUID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() updateCompanyRequest { return updateCompanyRequest{} },
		func(req updateCompanyRequest) (database.UpdateCompanyParams, error) {
			return database.UpdateCompanyParams{
				CompanyName: req.CompanyName,
				IsActive:    req.IsActive,
				ID:          companyID,
			}, nil
		},
		func(ctx context.Context, params database.UpdateCompanyParams) (database.Company, error) {
			return c.DB.UpdateCompany(ctx, params)
		},
		http.StatusOK,
	)
}

func (c *Company) DeleteById(w http.ResponseWriter, r *http.Request) {
	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusNotFound, "Failed to parse id to UUID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	err = c.DB.DeleteCompany(ctx, companyID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to get company by ID:", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, struct{}{})
}

func (c *Company) ResetDB(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	err := c.DB.ResetCompanies(ctx)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to get company by ID:", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, struct{}{})
}
