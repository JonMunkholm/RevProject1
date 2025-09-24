package handler

import (
	"context"
	"net/http"
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
	ProdName                        string `json:"ProdName"`
	RevAssessment                   string `json:"RevAssessment"`
	OverTimePercent                 string `json:"OverTimePercent"`
	PointInTimePercent              string `json:"PointInTimePercent"`
	StandaloneSellingPriceMethod    string `json:"StandaloneSellingPriceMethod"`
	StandaloneSellingPricePriceHigh string `json:"StandaloneSellingPricePriceHigh"`
	StandaloneSellingPricePriceLow  string `json:"StandaloneSellingPricePriceLow"`
	DefaultCurrency                 string `json:"DefaultCurrency"`
}

type updateProduct struct {
	ProdName                        string `json:"ProdName"`
	RevAssessment                   string `json:"RevAssessment"`
	OverTimePercent                 string `json:"OverTimePercent"`
	PointInTimePercent              string `json:"PointInTimePercent"`
	StandaloneSellingPriceMethod    string `json:"StandaloneSellingPriceMethod"`
	StandaloneSellingPricePriceHigh string `json:"StandaloneSellingPricePriceHigh"`
	StandaloneSellingPricePriceLow  string `json:"StandaloneSellingPricePriceLow"`
	DefaultCurrency                 string `json:"DefaultCurrency"`
	IsActive                        bool   `json:"IsActive"`
}

type companyProductsRequest struct {
	CompanyID uuid.UUID `json:"CompanyID"`
}

func (p *Product) Create(w http.ResponseWriter, r *http.Request) {

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
		func() createProduct { return createProduct{} },
		func(req createProduct) (database.CreateProductParams, error) {
			dbReq := database.CreateProductParams{
				CompanyID:                       companyID,
				ProdName:                        req.ProdName,
				RevAssessment:                   req.RevAssessment,
				OverTimePercent:                 req.OverTimePercent,
				PointInTimePercent:              req.PointInTimePercent,
				StandaloneSellingPriceMethod:    req.StandaloneSellingPriceMethod,
				StandaloneSellingPricePriceHigh: req.StandaloneSellingPricePriceHigh,
				StandaloneSellingPricePriceLow:  req.StandaloneSellingPricePriceLow,
				DefaultCurrency:                 req.DefaultCurrency,
			}

			payload := productPayload{
				CompanyID:          dbReq.CompanyID,
				ProdName:           dbReq.ProdName,
				RevAssessment:      dbReq.RevAssessment,
				OverTimePercent:    dbReq.OverTimePercent,
				PointInTimePercent: dbReq.PointInTimePercent,
				SSPMethod:          dbReq.StandaloneSellingPriceMethod,
				SSPHigh:            dbReq.StandaloneSellingPricePriceHigh,
				SSPLow:             dbReq.StandaloneSellingPricePriceLow,
				DefaultCurrency:    dbReq.DefaultCurrency,
			}

			if err := validateProductStrict(&payload); err != nil {
				return dbReq, err
			}

			dbReq.ProdName = payload.ProdName
			dbReq.RevAssessment = payload.RevAssessment
			dbReq.OverTimePercent = payload.OverTimePercent
			dbReq.PointInTimePercent = payload.PointInTimePercent
			dbReq.StandaloneSellingPriceMethod = payload.SSPMethod
			dbReq.StandaloneSellingPricePriceHigh = payload.SSPHigh
			dbReq.StandaloneSellingPricePriceLow = payload.SSPLow
			dbReq.DefaultCurrency = payload.DefaultCurrency

			return dbReq, nil
		},
		func(ctx context.Context, params database.CreateProductParams) (database.Product, error) {
			return p.DB.CreateProduct(ctx, params)
		},
		http.StatusCreated,
	)

}

func (p *Product) List(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, param uuid.UUID) ([]database.Product, error) {
			return p.DB.GetAllProductsCompany(ctx, param)
		},
		http.StatusOK,
	)

}

func (p *Product) ListAll(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, param interface{}) ([]database.Product, error) {
			return p.DB.GetAllProducts(ctx)
		},
		http.StatusOK,
	)

}

func (p *Product) GetActive(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, param uuid.UUID) ([]database.Product, error) {
			return p.DB.GetActiveProductsCompany(ctx, param)
		},
		http.StatusOK,
	)

}

func (p *Product) GetById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	productID, err := uuid.Parse(chi.URLParam(r, "productID"))
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

func (p *Product) GetByName(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	productName := strings.TrimSpace(chi.URLParam(r, "productName"))

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() companyProductsRequest { return companyProductsRequest{} },
		func(req companyProductsRequest) (database.GetProductByNameParams, error) {
			return database.GetProductByNameParams{
				ProdName:  productName,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetProductByNameParams) (database.Product, error) {
			return p.DB.GetProductByName(ctx, param)
		},
		http.StatusOK,
	)

}

func (p *Product) UpdateById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	productID, err := uuid.Parse(chi.URLParam(r, "productID"))
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
		func() updateProduct { return updateProduct{} },
		func(req updateProduct) (database.UpdateProductParams, error) {
			dbReq := database.UpdateProductParams{
				ProdName:                        req.ProdName,
				RevAssessment:                   req.RevAssessment,
				OverTimePercent:                 req.OverTimePercent,
				PointInTimePercent:              req.PointInTimePercent,
				StandaloneSellingPriceMethod:    req.StandaloneSellingPriceMethod,
				StandaloneSellingPricePriceHigh: req.StandaloneSellingPricePriceHigh,
				StandaloneSellingPricePriceLow:  req.StandaloneSellingPricePriceLow,
				DefaultCurrency:                 req.DefaultCurrency,
				IsActive:                        req.IsActive,
				ID:                              productID,
				CompanyID:                       companyID,
			}

			payload := productPayload{
				CompanyID:          dbReq.CompanyID,
				ProdName:           dbReq.ProdName,
				RevAssessment:      dbReq.RevAssessment,
				OverTimePercent:    dbReq.OverTimePercent,
				PointInTimePercent: dbReq.PointInTimePercent,
				SSPMethod:          dbReq.StandaloneSellingPriceMethod,
				SSPHigh:            dbReq.StandaloneSellingPricePriceHigh,
				SSPLow:             dbReq.StandaloneSellingPricePriceLow,
				DefaultCurrency:    dbReq.DefaultCurrency,
			}

			if err := validateProductStrict(&payload); err != nil {
				return dbReq, err
			}

			dbReq.ProdName = payload.ProdName
			dbReq.RevAssessment = payload.RevAssessment
			dbReq.OverTimePercent = payload.OverTimePercent
			dbReq.PointInTimePercent = payload.PointInTimePercent
			dbReq.StandaloneSellingPriceMethod = payload.SSPMethod
			dbReq.StandaloneSellingPricePriceHigh = payload.SSPHigh
			dbReq.StandaloneSellingPricePriceLow = payload.SSPLow
			dbReq.DefaultCurrency = payload.DefaultCurrency

			return dbReq, nil
		},
		func(ctx context.Context, params database.UpdateProductParams) (database.Product, error) {
			return p.DB.UpdateProduct(ctx, params)
		},
		http.StatusOK,
	)

}

func (p *Product) SetActive(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	productID, err := uuid.Parse(chi.URLParam(r, "productID"))
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
		func() setActiveRequest { return setActiveRequest{} },
		func(req setActiveRequest) (database.SetProductActiveStatusParams, error) {
			return database.SetProductActiveStatusParams{
				IsActive:  req.IsActive,
				ID:        productID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.SetProductActiveStatusParams) (struct{}, error) {
			return struct{}{}, p.DB.SetProductActiveStatus(ctx, param)
		},
		http.StatusOK,
	)

}

func (p *Product) DeleteById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	productID, err := uuid.Parse(chi.URLParam(r, "productID"))
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
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.DeleteProductParams, error) {
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

func (p *Product) ResetCompanyTable(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, param uuid.UUID) (struct{}, error) {
			return struct{}{}, p.DB.DeleteAllProductsCompany(ctx, param)
		},
		http.StatusOK,
	)
}

func (p *Product) ResetTable(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, param interface{}) (struct{}, error) {
			return struct{}{}, p.DB.ResetProducts(ctx)
		},
		http.StatusOK,
	)
}
