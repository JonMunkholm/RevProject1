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

type Customer struct {
	DB *database.Queries
}

type createCustomerRequest struct {
	CustomerName string `json:"CustomerName"`
}

type updateCustomerRequest struct {
	CustomerName string `json:"CustomerName"`
	IsActive     bool   `json:"IsActive"`
}

func (c *Customer) Create(w http.ResponseWriter, r *http.Request) {

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
		func() createCustomerRequest { return createCustomerRequest{} },
		func(req createCustomerRequest) (database.CreateCustomerParams, error) {
			return database.CreateCustomerParams{
				CustomerName: req.CustomerName,
				CompanyID:    companyID,
			}, nil
		},
		func(ctx context.Context, params database.CreateCustomerParams) (database.Customer, error) {
			return c.DB.CreateCustomer(ctx, params)
		},
		http.StatusCreated,
	)

}

func (c *Customer) List(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, param uuid.UUID) ([]database.Customer, error) {
			return c.DB.GetAllCustomersCompany(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Customer) ListAll(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	companies, err := c.DB.GetAllCustomers(ctx)
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

func (c *Customer) GetById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	customerID, err := uuid.Parse(chi.URLParam(r, "customerID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid customer ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetCustomerParams, error) {
			return database.GetCustomerParams{
				ID:        customerID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetCustomerParams) (database.Customer, error) {
			return c.DB.GetCustomer(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Customer) GetByName(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	customerName := strings.TrimSpace(chi.URLParam(r, "name"))

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetCustomerByNameParams, error) {
			return database.GetCustomerByNameParams{
				CompanyID:    companyID,
				CustomerName: customerName,
			}, nil
		},
		func(ctx context.Context, param database.GetCustomerByNameParams) (database.Customer, error) {
			return c.DB.GetCustomerByName(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Customer) GetActive(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, param uuid.UUID) ([]database.Customer, error) {
			return c.DB.GetActiveCustomersCompany(ctx, param)
		},
		http.StatusOK,
	)
}

func (c *Customer) SetActive(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	customerID, err := uuid.Parse(chi.URLParam(r, "customerID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid customer ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() setActiveRequest { return setActiveRequest{} },
		func(req setActiveRequest) (database.SetCustomerActiveStatusParams, error) {
			return database.SetCustomerActiveStatusParams{
				IsActive:  req.IsActive,
				ID:        customerID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.SetCustomerActiveStatusParams) (struct{}, error) {
			return struct{}{}, c.DB.SetCustomerActiveStatus(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Customer) UpdateById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	customerID, err := uuid.Parse(chi.URLParam(r, "customerID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid customer ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() updateCustomerRequest { return updateCustomerRequest{} },
		func(req updateCustomerRequest) (database.UpdateCustomerParams, error) {
			return database.UpdateCustomerParams{
				CustomerName: req.CustomerName,
				IsActive:     req.IsActive,
				ID:           customerID,
				CompanyID:    companyID,
			}, nil
		},
		func(ctx context.Context, param database.UpdateCustomerParams) (database.Customer, error) {
			return c.DB.UpdateCustomer(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Customer) DeleteById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	customerID, err := uuid.Parse(chi.URLParam(r, "customerID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid customer ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.DeleteCustomerParams, error) {
			return database.DeleteCustomerParams{
				ID:        customerID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.DeleteCustomerParams) (struct{}, error) {
			return struct{}{}, c.DB.DeleteCustomer(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Customer) ResetTable(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	err := c.DB.ResetCustomers(ctx)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to get company by ID:", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, struct{}{})
}
