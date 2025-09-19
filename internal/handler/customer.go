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


type Customer struct {
	DB *database.Queries
}

type createCustomerRequest struct {
    CustomerName  	string		`json:"CustomerName"`
	CompanyID 		uuid.UUID	`json:"CompanyID"`
}

func (c *Customer) Create (w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    _, _ = createRecord(
        ctx,
        w,
        r,
        func() createCustomerRequest { return createCustomerRequest{} },
        func(req createCustomerRequest) (database.CreateCustomerParams, error) {
            return database.CreateCustomerParams{
                CustomerName:  	req.CustomerName,
                CompanyID: 		req.CompanyID,
            }, nil
        },
        func(ctx context.Context, params database.CreateCustomerParams) (database.Customer, error) {
            return c.DB.CreateCustomer(ctx, params)
        },
        http.StatusCreated,
    )

}

func (c *Customer) List (w http.ResponseWriter, r *http.Request) {
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

	customers, err := c.DB.GetAllCustomersCompany(ctx, request.CompanyID)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create customers:", err)
		return
	}

	data, err := json.Marshal(customers)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (c *Customer) GetById (w http.ResponseWriter, r *http.Request) {
	customerIDString := chi.URLParam(r,"id")

	customerID, err := uuid.Parse(customerIDString)
	if err != nil {
		RespondWithError(w, http.StatusNotFound,"Error missing or invalid customer ID:", err)
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

	dbReq := database.GetCustomerParams{
		ID: customerID,
		CompanyID: request.CompanyID,
	}
	customer, err := c.DB.GetCustomer(ctx, dbReq)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create customer:", err)
		return
	}

	data, err := json.Marshal(customer)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (c *Customer) UpdateById (w http.ResponseWriter, r *http.Request) {
	//no query support for this currently
}

func (c *Customer) DeleteById (w http.ResponseWriter, r *http.Request) {
	customerIDString := chi.URLParam(r,"id")

	customerID, err := uuid.Parse(customerIDString)
	if err != nil {
		RespondWithError(w, http.StatusNotFound,"Error missing or invalid customer ID:", err)
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

	dbReq := database.DeleteCustomerParams{
		ID: customerID,
		CompanyID: request.CompanyID,
	}

	err = c.DB.DeleteCustomer(ctx, dbReq)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create customer:", err)
		return
	}


	w.WriteHeader(http.StatusOK)
}
