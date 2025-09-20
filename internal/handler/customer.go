package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type Customer struct {
	DB *database.Queries
}

type createCustomerRequest struct {
	CustomerName string    `json:"CustomerName"`
	CompanyID    uuid.UUID `json:"CompanyID"`
}

type companyCustomersRequest struct {
	CompanyID uuid.UUID `json:"CompanyID"`
}

func (c *Customer) Create(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() createCustomerRequest { return createCustomerRequest{} },
		func(req createCustomerRequest) (database.CreateCustomerParams, error) {
			companyID, err := companyIDFromRequest(req.CompanyID)
			if err != nil {
				return database.CreateCustomerParams{}, err
			}

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

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() companyCustomersRequest { return companyCustomersRequest{} },
		func(req companyCustomersRequest) (uuid.UUID, error) {
			return companyIDFromRequest(req.CompanyID)
		},
		func(ctx context.Context, param uuid.UUID) ([]database.Customer, error) {
			return c.DB.GetAllCustomersCompany(ctx, param)
		},
		http.StatusOK,
	)

}

func (c *Customer) GetById(w http.ResponseWriter, r *http.Request) {
	customerIDString := chi.URLParam(r, "id")

	customerID, err := uuid.Parse(customerIDString)
	if err != nil {
		RespondWithError(w, http.StatusNotFound, "Error missing or invalid customer ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() companyCustomersRequest { return companyCustomersRequest{} },
		func(req companyCustomersRequest) (database.GetCustomerParams, error) {
			companyID, err := companyIDFromRequest(req.CompanyID)
			if err != nil {
				return database.GetCustomerParams{}, err
			}

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

func (c *Customer) UpdateById(w http.ResponseWriter, r *http.Request) {
	//no query support for this currently
}

func (c *Customer) DeleteById(w http.ResponseWriter, r *http.Request) {
	customerIDString := chi.URLParam(r, "id")

	customerID, err := uuid.Parse(customerIDString)
	if err != nil {
		RespondWithError(w, http.StatusNotFound, "Error missing or invalid customer ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() companyCustomersRequest { return companyCustomersRequest{} },
		func(req companyCustomersRequest) (database.DeleteCustomerParams, error) {
			companyID, err := companyIDFromRequest(req.CompanyID)
			if err != nil {
				return database.DeleteCustomerParams{}, err
			}

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
