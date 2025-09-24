package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type Contract struct {
	DB *database.Queries
}

type createContract struct {
	CustomerID  uuid.UUID `json:"CustomerID"`
	StartDate   time.Time `json:"StartDate"`
	EndDate     time.Time `json:"EndDate"`
	IsFinal     bool      `json:"IsFinal"`
	ContractUrl *string   `json:"ContractUrl"`
}

type updateContract struct {
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

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

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
				contractURL = sql.NullString{String: *req.ContractUrl, Valid: true}
			}

			dbReq := database.CreateContractParams{
				CompanyID:   companyID,
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

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
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
		func(ctx context.Context, param uuid.UUID) ([]database.Contract, error) {
			return c.DB.GetAllContractsCompany(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Contract) ListCustomer(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	customerID, err := uuid.Parse(chi.URLParam(r, "customerID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid customer ID", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetContractsByCustomerParams, error) {
			return database.GetContractsByCustomerParams{
				CompanyID:  companyID,
				CustomerID: customerID,
			}, nil
		},
		func(ctx context.Context, params database.GetContractsByCustomerParams) ([]database.Contract, error) {
			return c.DB.GetContractsByCustomer(ctx, params)
		},
		http.StatusOK,
	)

}

func (c *Contract) ListAll(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (interface{}, error) {
			return struct{}{}, nil
		},
		func(ctx context.Context, param interface{}) ([]database.Contract, error) {
			return c.DB.GetAllContracts(ctx)
		},
		http.StatusOK,
	)

}

func (c *Contract) GetById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	contractID, err := uuid.Parse(chi.URLParam(r, "contractID"))
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

func (c *Contract) GetFinal(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (uuid.UUID, error) {
			return companyID, nil
		},
		func(ctx context.Context, param uuid.UUID) ([]database.Contract, error) {
			return c.DB.GetFinalContractsCompany(ctx, param)
		},
		http.StatusOK,
	)
}

func (c *Contract) UpdateById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	contractID, err := uuid.Parse(chi.URLParam(r, "contractID"))
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
		func() updateContract { return updateContract{} },
		func(req updateContract) (database.UpdateContractParams, error) {
			contractURL := sql.NullString{}
			if req.ContractUrl != nil {
				contractURL = sql.NullString{String: *req.ContractUrl, Valid: true}
			}

			dbReq := database.UpdateContractParams{
				CustomerID:  req.CustomerID,
				StartDate:   req.StartDate,
				EndDate:     req.EndDate,
				IsFinal:     req.IsFinal,
				ContractUrl: contractURL,
				ID:          contractID,
				CompanyID:   companyID,
			}

			if err := c.contractUpdateValidation(&dbReq); err != nil {
				return dbReq, err
			}

			return dbReq, nil
		},
		func(ctx context.Context, param database.UpdateContractParams) (database.Contract, error) {
			return c.DB.UpdateContract(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Contract) DeleteById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	contractID, err := uuid.Parse(chi.URLParam(r, "contractID"))
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

func (c *Contract) ResetTable(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	err := c.DB.ResetContracts(ctx)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to reset contracts table", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, struct{}{})
}
