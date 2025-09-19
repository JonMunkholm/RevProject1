package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type Product struct {
	DB *database.Queries
}

func (p *Product) Create(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	request := struct {
		CompanyID                       uuid.UUID `json:"CompanyID"`
		ProdName                        string    `json:"ProdName"`
		RevAssessment                   string    `json:"RevAssessment"`
		OverTimePercent                 string    `json:"OverTimePercent"`
		PointInTimePercent              string    `json:"PointInTimePercent"`
		StandaloneSellingPriceMethod    string    `json:"StandaloneSellingPriceMethod"`
		StandaloneSellingPricePriceHigh string    `json:"StandaloneSellingPricePriceHigh"`
		StandaloneSellingPricePriceLow  string    `json:"StandaloneSellingPricePriceLow"`
		DefaultCurrency                 string    `json:"DefaultCurrency"`
	}{}

	err := decoder.Decode(&request)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error decoding request", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	dbReq := database.CreateProductParams{
		CompanyID:                       request.CompanyID,
		ProdName:                        request.ProdName,
		RevAssessment:                   request.RevAssessment,
		OverTimePercent:                 request.OverTimePercent,
		PointInTimePercent:              request.PointInTimePercent,
		StandaloneSellingPriceMethod:    request.StandaloneSellingPriceMethod,
		StandaloneSellingPricePriceHigh: request.StandaloneSellingPricePriceHigh,
		StandaloneSellingPricePriceLow:  request.StandaloneSellingPricePriceLow,
		DefaultCurrency:                 request.DefaultCurrency,
	}
	if err := p.productInputValidation(&dbReq); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid product payload:", err)
		return
	}

	Products, err := p.DB.CreateProduct(ctx, dbReq)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create Products:", err)
		return
	}

	data, err := json.Marshal(Products)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(data)
}

func (p *Product) List(w http.ResponseWriter, r *http.Request) {
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

	products, err := p.DB.GetAllProductsCompany(ctx, request.CompanyID)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create products:", err)
		return
	}

	data, err := json.Marshal(products)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (p *Product) GetById(w http.ResponseWriter, r *http.Request) {
	productIDString := chi.URLParam(r, "id")

	productID, err := uuid.Parse(productIDString)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid product ID:", err)
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

	dbReq := database.GetProductParams{
		ID: 		   productID,
		CompanyID:     request.CompanyID,
	}
	Products, err := p.DB.GetProduct(ctx, dbReq)

	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create Products:", err)
		return
	}

	data, err := json.Marshal(Products)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Error marshaling response:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func (p *Product) UpdateById(w http.ResponseWriter, r *http.Request) {
	//no query support for this currently
}

func (p *Product) DeleteById(w http.ResponseWriter, r *http.Request) {
	productIDString := chi.URLParam(r, "id")

	productID, err := uuid.Parse(productIDString)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid product ID:", err)
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

	dbReq := database.DeleteProductParams{
		ID: 		   productID,
		CompanyID:     request.CompanyID,
	}
	err = p.DB.DeleteProduct(ctx, dbReq)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create Products:", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (p *Product) productInputValidation(prod *database.CreateProductParams) error {
	//-------------------Additional validation logic needed:-------------------
	if prod == nil {
		return fmt.Errorf("missing product payload")
	}

	if prod.CompanyID == uuid.Nil {
		return fmt.Errorf("CompanyID is required")
	}

	prod.ProdName = strings.TrimSpace(prod.ProdName)
	if prod.ProdName == "" {
		return fmt.Errorf("ProdName is required")
	}
	if len(prod.ProdName) > 255 {
		return fmt.Errorf("ProdName exceeds 255 characters")
	}

	allowedAssessments := map[string]struct{}{
		"over_time":     {},
		"point_in_time": {},
		"split":         {},
	}
	if _, ok := allowedAssessments[prod.RevAssessment]; !ok {
		return fmt.Errorf("RevAssessment must be one of over_time, point_in_time, split")
	}

	parsePercent := func(label, value string) (float64, error) {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("%s must be a numeric value", label)
		}
		if parsed < 0 || parsed > 1 {
			return 0, fmt.Errorf("%s must be between 0.0000 and 1.0000", label)
		}
		return parsed, nil
	}

	overTimePercent, err := parsePercent("OverTimePercent", prod.OverTimePercent)
	if err != nil {
		return err
	}

	pointInTimePercent, err := parsePercent("PointInTimePercent", prod.PointInTimePercent)
	if err != nil {
		return err
	}

	allowedSSPMethods := map[string]struct{}{
		"observable":      {},
		"adjusted_market": {},
		"cost_plus":       {},
		"residual":        {},
	}
	if _, ok := allowedSSPMethods[prod.StandaloneSellingPriceMethod]; !ok {
		return fmt.Errorf("StandaloneSellingPriceMethod must be one of observable, adjusted_market, cost_plus, residual")
	}

	floatEquals := func(a, b float64) bool {
		return math.Abs(a-b) <= 1e-9
	}

	switch prod.RevAssessment {
	case "over_time":
		if !floatEquals(overTimePercent, 1) {
			return fmt.Errorf("OverTimePercent must equal 1.0000 when RevAssessment is over_time")
		}
		if !floatEquals(pointInTimePercent, 0) {
			return fmt.Errorf("PointInTimePercent must equal 0.0000 when RevAssessment is over_time")
		}
	case "point_in_time":
		if !floatEquals(pointInTimePercent, 1) {
			return fmt.Errorf("PointInTimePercent must equal 1.0000 when RevAssessment is point_in_time")
		}
		if !floatEquals(overTimePercent, 0) {
			return fmt.Errorf("OverTimePercent must equal 0.0000 when RevAssessment is point_in_time")
		}
	case "split":
		sum := overTimePercent + pointInTimePercent
		if math.Abs(sum-1) > 1e-6 {
			return fmt.Errorf("OverTimePercent plus PointInTimePercent must equal 1.0000 when RevAssessment is split")
		}
	}

	parsePrice := func(label, value string) (float64, error) {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("%s must be a numeric value", label)
		}
		if parsed < 0 {
			return 0, fmt.Errorf("%s must be greater than or equal to zero", label)
		}
		return parsed, nil
	}

	high, err := parsePrice("StandaloneSellingPricePriceHigh", prod.StandaloneSellingPricePriceHigh)
	if err != nil {
		return err
	}

	low, err := parsePrice("StandaloneSellingPricePriceLow", prod.StandaloneSellingPricePriceLow)
	if err != nil {
		return err
	}

	if high < low {
		return fmt.Errorf("StandaloneSellingPricePriceHigh must be greater than or equal to StandaloneSellingPricePriceLow")
	}

	prod.DefaultCurrency = strings.TrimSpace(strings.ToUpper(prod.DefaultCurrency))
	if len(prod.DefaultCurrency) != 3 {
		return fmt.Errorf("DefaultCurrency must be a 3-letter ISO code")
	}

	return nil
}
