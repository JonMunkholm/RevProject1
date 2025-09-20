package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type Contract struct {
	DB *database.Queries
}

type createContract struct {
	CompanyID   uuid.UUID `json:"CompanyID"`
	CustomerID  uuid.UUID `json:"CustomerID"`
	StartDate   time.Time `json:"StartDate"`
	EndDate     time.Time `json:"EndDate"`
	IsFinal     bool      `json:"IsFinal"`
	ContractUrl *string   `json:"ContractUrl"`
}

type companyContractsRequest struct {
	CompanyID uuid.UUID `json:"CompanyID"`
}

func (c *Contract) Create(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() createContract { return createContract{} },
		func(req createContract) (database.CreateContractParams, error) {
			contractURL := sql.NullString{}
			if req.ContractUrl != nil {
				trimmed := strings.TrimSpace(*req.ContractUrl)
				if trimmed != "" {
					contractURL = sql.NullString{String: trimmed, Valid: true}
				}
			}

			dbReq := database.CreateContractParams{
				CompanyID:   req.CompanyID,
				CustomerID:  req.CustomerID,
				StartDate:   req.StartDate,
				EndDate:     req.EndDate,
				IsFinal:     req.IsFinal,
				ContractUrl: contractURL,
			}

			if err := c.contractInputValidation(&dbReq); err != nil {
				return dbReq, err
			}
			return dbReq, nil
		},
		func(ctx context.Context, params database.CreateContractParams) (database.Contract, error) {
			return c.DB.CreateContract(ctx, params)
		},
		http.StatusCreated,
	)

}

func (c *Contract) List(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() companyContractsRequest { return companyContractsRequest{} },
		func(req companyContractsRequest) (uuid.UUID, error) {
			return companyIDFromRequest(req.CompanyID)
		},
		func(ctx context.Context, param uuid.UUID) ([]database.Contract, error) {
			return c.DB.GetAllContractsCompany(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Contract) GetById(w http.ResponseWriter, r *http.Request) {

	contractID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid contract ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() companyContractsRequest { return companyContractsRequest{} },
		func(req companyContractsRequest) (database.GetContractParams, error) {
			companyID, err := companyIDFromRequest(req.CompanyID)
			if err != nil {
				return database.GetContractParams{}, err
			}

			return database.GetContractParams{
				ID:        contractID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetContractParams) (database.Contract, error) {
			return c.DB.GetContract(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Contract) UpdateById(w http.ResponseWriter, r *http.Request) {
	//no query support for this currently
}

func (c *Contract) DeleteById(w http.ResponseWriter, r *http.Request) {

	contractID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid contract ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() companyContractsRequest { return companyContractsRequest{} },
		func(req companyContractsRequest) (database.DeleteContractParams, error) {
			companyID, err := companyIDFromRequest(req.CompanyID)
			if err != nil {
				return database.DeleteContractParams{}, err
			}

			return database.DeleteContractParams{
				ID:        contractID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.DeleteContractParams) (struct{}, error) {
			return struct{}{}, c.DB.DeleteContract(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Contract) contractInputValidation(ct *database.CreateContractParams) error {
	if ct == nil {
		return fmt.Errorf("missing contract payload")
	}

	if ct.CompanyID == uuid.Nil {
		return fmt.Errorf("CompanyID is required")
	}

	if ct.CustomerID == uuid.Nil {
		return fmt.Errorf("CustomerID is required")
	}

	if ct.StartDate.IsZero() {
		return fmt.Errorf("StartDate is required")
	}

	if ct.EndDate.IsZero() {
		return fmt.Errorf("EndDate is required")
	}

	if !ct.EndDate.After(ct.StartDate) {
		return fmt.Errorf("EndDate must be after StartDate")
	}

	if ct.ContractUrl.Valid {
		trimmed := strings.TrimSpace(ct.ContractUrl.String)
		if trimmed == "" {
			ct.ContractUrl.Valid = false
		} else {
			ct.ContractUrl.String = trimmed
		}
	}

	return nil
}
