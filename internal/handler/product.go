package handler

import (
	"context"
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

type createProduct struct {
	CompanyID                       uuid.UUID `json:"CompanyID"`
	ProdName                        string    `json:"ProdName"`
	RevAssessment                   string    `json:"RevAssessment"`
	OverTimePercent                 string    `json:"OverTimePercent"`
	PointInTimePercent              string    `json:"PointInTimePercent"`
	StandaloneSellingPriceMethod    string    `json:"StandaloneSellingPriceMethod"`
	StandaloneSellingPricePriceHigh string    `json:"StandaloneSellingPricePriceHigh"`
	StandaloneSellingPricePriceLow  string    `json:"StandaloneSellingPricePriceLow"`
	DefaultCurrency                 string    `json:"DefaultCurrency"`
}

type companyProductsRequest struct {
	CompanyID uuid.UUID `json:"CompanyID"`
}

func (p *Product) Create(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() createProduct { return createProduct{} },
		func(req createProduct) (database.CreateProductParams, error) {
			dbReq := database.CreateProductParams{
				CompanyID:                       req.CompanyID,
				ProdName:                        req.ProdName,
				RevAssessment:                   req.RevAssessment,
				OverTimePercent:                 req.OverTimePercent,
				PointInTimePercent:              req.PointInTimePercent,
				StandaloneSellingPriceMethod:    req.StandaloneSellingPriceMethod,
				StandaloneSellingPricePriceHigh: req.StandaloneSellingPricePriceHigh,
				StandaloneSellingPricePriceLow:  req.StandaloneSellingPricePriceLow,
				DefaultCurrency:                 req.DefaultCurrency,
			}

			if err := p.productInputValidation(&dbReq); err != nil {
				return dbReq, err
			}
			return dbReq, nil
		},
		func(ctx context.Context, params database.CreateProductParams) (database.Product, error) {
			return p.DB.CreateProduct(ctx, params)
		},
		http.StatusCreated,
	)

}

func (p *Product) List(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() companyProductsRequest { return companyProductsRequest{} },
		func(req companyProductsRequest) (uuid.UUID, error) {
			return companyIDFromRequest(req.CompanyID)
		},
		func(ctx context.Context, param uuid.UUID) ([]database.Product, error) {
			return p.DB.GetAllProductsCompany(ctx, param)
		},
		http.StatusOK,
	)

}

func (p *Product) GetById(w http.ResponseWriter, r *http.Request) {

	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid product ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() companyProductsRequest { return companyProductsRequest{} },
		func(req companyProductsRequest) (database.GetProductParams, error) {
			companyID, err := companyIDFromRequest(req.CompanyID)
			if err != nil {
				return database.GetProductParams{}, err
			}

			return database.GetProductParams{
				ID:        productID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetProductParams) (database.Product, error) {
			return p.DB.GetProduct(ctx, param)
		},
		http.StatusOK,
	)

}

func (p *Product) UpdateById(w http.ResponseWriter, r *http.Request) {
	//no query support for this currently
}

func (p *Product) DeleteById(w http.ResponseWriter, r *http.Request) {

	productID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid product ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() companyProductsRequest { return companyProductsRequest{} },
		func(req companyProductsRequest) (database.DeleteProductParams, error) {
			companyID, err := companyIDFromRequest(req.CompanyID)
			if err != nil {
				return database.DeleteProductParams{}, err
			}

			return database.DeleteProductParams{
				ID:        productID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.DeleteProductParams) (struct{}, error) {
			return struct{}{}, p.DB.DeleteProduct(ctx, param)
		},
		http.StatusOK,
	)

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
