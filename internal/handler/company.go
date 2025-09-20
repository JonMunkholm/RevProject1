package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)


type Company struct {
	DB *database.Queries
}

type createCompanyRequest struct {
    CompanyName string `json:"CompanyName"`
    UserName    string `json:"UserName"`
}

type createCompanyArgs struct {
    CompanyName string
    UserName    string
}

type createCompanyResponse struct {
    Company database.Company `json:"company"`
    User    database.User    `json:"user"`
}

func (c *Company) Create (w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    _, _ = processRequest(
        ctx,
        w,
        r,
        func() createCompanyRequest { return createCompanyRequest{} },
        func(req createCompanyRequest) (createCompanyArgs, error) {
            companyName := strings.TrimSpace(req.CompanyName)
            userName := strings.TrimSpace(req.UserName)

            switch {
            case companyName == "":
                return createCompanyArgs{}, errors.New("CompanyName is required")
            case userName == "":
                return createCompanyArgs{}, errors.New("UserName is required")
            default:
                return createCompanyArgs{
                    CompanyName: companyName,
                    UserName:    userName,
                }, nil
            }
        },
        func(ctx context.Context, params createCompanyArgs) (createCompanyResponse, error) {
            company, err := c.DB.CreateCompany(ctx, params.CompanyName)
            if err != nil {
                return createCompanyResponse{}, err
            }

            user, err := c.DB.CreateUser(ctx, database.CreateUserParams{
                UserName:  params.UserName,
                CompanyID: company.ID,
            })
            if err != nil {
                return createCompanyResponse{}, err
            }

            return createCompanyResponse{
                Company: company,
                User:    user,
            }, nil
        },
        http.StatusCreated,
    )
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
	companyID, err := uuid.Parse(chi.URLParam(r,"id"))
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

	RespondWithJSON(w, http.StatusOK, company)
}

func (c *Company) UpdateById (w http.ResponseWriter, r *http.Request) {
	//no query support for this currently
}

func (c *Company) DeleteById (w http.ResponseWriter, r *http.Request) {
	companyID, err := uuid.Parse(chi.URLParam(r,"id"))
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

	RespondWithJSON(w, http.StatusOK, struct{}{})
}
