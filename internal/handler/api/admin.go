package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
)

type Admin struct {
	DB *database.Queries
}

const domain = "http://localhost:8080"
const coName = "Another Test Co."
const userEmail = "tick.tock@example.com"
const userPassword = "changeme123"
const custName = "One More Time LLC."

func (u *Admin) QuickStart(w http.ResponseWriter, r *http.Request) {
	//Create company and first user for the company
	newCo := struct {
		CompanyName string `json:"CompanyName"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}{
		CompanyName: coName,
		Email:       userEmail,
		Password:    userPassword,
	}
	urlNewCo := fmt.Sprintf("%v/companies", domain)

	createCoResp, err := u.createNewRecord(r.Context(), newCo, urlNewCo)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create new company:", err)
		return
	}

	// top-level object
	m, ok := createCoResp.(map[string]any)
	if !ok {
		RespondWithError(w, http.StatusInternalServerError, "", errors.New("expected object at top level"))
		return
	}

	// nested object(s)
	company, ok := m["company"].(map[string]any)
	if !ok {
		RespondWithError(w, http.StatusInternalServerError, "", errors.New("missing company field"))
		return
	}

	user, ok := m["user"].(map[string]any)
	if !ok {
		RespondWithError(w, http.StatusInternalServerError, "", errors.New("missing user field"))
		return
	}

	// field value
	companyID, ok := company["ID"].(string)
	if !ok {
		RespondWithError(w, http.StatusInternalServerError, "", errors.New("missing CompanyID field"))
		return
	}

	//pipe output from creating company into creating first customer of company
	newCust := struct {
		CompanyID    string `json:"CompanyID"`
		CustomerName string `json:"CustomerName"`
	}{
		CompanyID:    companyID,
		CustomerName: custName,
	}
	urlNewCust := fmt.Sprintf("%v/customers", domain)

	createCustResp, err := u.createNewRecord(r.Context(), newCust, urlNewCust)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create new customer:", err)
		return
	}

	data, err := json.Marshal(map[string]interface{}{"company": company, "user": user, "customer": createCustResp})
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to marshal data:", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(data)

}

func (u *Admin) Reset(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	err := u.DB.ResetCompanies(ctx)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to get reset DB:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (u *Admin) createNewRecord(ctx context.Context, reqBody interface{}, url string) (interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*20)
	defer cancel()

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return struct{}{}, fmt.Errorf("failed to marshal newCo request: %w", err)
	}

	bodyReader := bytes.NewBuffer(jsonBody)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bodyReader)
	if err != nil {
		return struct{}{}, fmt.Errorf("failed to create newCo http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return struct{}{}, fmt.Errorf("failed newCo http request: %w", err)
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)

	var respJson interface{}

	err = decoder.Decode(&respJson)
	if err != nil {
		return struct{}{}, fmt.Errorf("failed to decode newCo response JSON: %w", err)
	}

	return respJson, nil
}
