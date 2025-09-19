package handler

import (
	"context"
	"database/sql"
	"encoding/json"
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

func (c *Contract) Create(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	request := struct {
		CompanyID   uuid.UUID      `json:"CompanyID"`
		CustomerID  uuid.UUID      `json:"CustomerID"`
		StartDate   time.Time      `json:"StartDate"`
		EndDate     time.Time      `json:"EndDate"`
		IsFinal     bool           `json:"IsFinal"`
		ContractUrl sql.NullString `json:"ContractUrl"`
	}{}

	err := decoder.Decode(&request)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error decoding request", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	dbReq := database.CreateContractParams{
		CompanyID:   request.CompanyID,
		CustomerID:  request.CustomerID,
		StartDate:   request.StartDate,
		EndDate:     request.EndDate,
		IsFinal:     request.IsFinal,
		ContractUrl: request.ContractUrl,
	}
	if err := c.contractInputValidation(&dbReq); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid contract payload:", err)
		return
	}

	contracts, err := c.DB.CreateContract(ctx, dbReq)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create contracts:", err)
		return
	}

	data, err := json.Marshal(contracts)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(data)
}

func (c *Contract) List(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	request := struct {
		CompanyID uuid.UUID `json:"CompanyID"`
	}{}

	err := decoder.Decode(&request)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error decoding request", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	contracts, err := c.DB.GetAllContractsCompany(ctx, request.CompanyID)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create contracts:", err)
		return
	}

	data, err := json.Marshal(contracts)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (c *Contract) GetById(w http.ResponseWriter, r *http.Request) {
	contractIDString := chi.URLParam(r, "id")

	contractID, err := uuid.Parse(contractIDString)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid contract ID:", err)
		return
	}

	decoder := json.NewDecoder(r.Body)
	request := struct {
		CompanyID uuid.UUID `json:"CompanyID"`
	}{}

	err = decoder.Decode(&request)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error decoding request", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	dbReq := database.GetContractParams{
		ID:			 contractID,
		CompanyID:   request.CompanyID,
	}
	contracts, err := c.DB.GetContract(ctx, dbReq)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create contracts:", err)
		return
	}

	data, err := json.Marshal(contracts)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (c *Contract) UpdateById(w http.ResponseWriter, r *http.Request) {
	//no query support for this currently
}

func (c *Contract) DeleteById(w http.ResponseWriter, r *http.Request) {
	contractIDString := chi.URLParam(r, "id")

	contractID, err := uuid.Parse(contractIDString)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid contract ID:", err)
		return
	}

	decoder := json.NewDecoder(r.Body)
	request := struct {
		CompanyID uuid.UUID `json:"CompanyID"`
	}{}

	err = decoder.Decode(&request)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error decoding request", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	dbReq := database.DeleteContractParams{
		ID:			 contractID,
		CompanyID:   request.CompanyID,
	}

	err = c.DB.DeleteContract(ctx, dbReq)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create contracts:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
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
