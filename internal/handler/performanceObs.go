package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type PerformanceObligation struct {
	DB *database.Queries
}

type createPerformanceObligation struct {
	PerformanceObligationsName string    `json:"PerformanceObligationsName"`
	StartDate                  time.Time `json:"StartDate"`
	EndDate                    time.Time `json:"EndDate"`
	FunctionalCurrency         string    `json:"FunctionalCurrency"`
	Discount                   string    `json:"Discount"`
	TransactionPrice           int64     `json:"TransactionPrice"`
}

type updatePerformanceObligation struct {
	PerformanceObligationsName string    `json:"PerformanceObligationsName"`
	StartDate                  time.Time `json:"StartDate"`
	EndDate                    time.Time `json:"EndDate"`
	FunctionalCurrency         string    `json:"FunctionalCurrency"`
	Discount                   string    `json:"Discount"`
	TransactionPrice           int64     `json:"TransactionPrice"`
}

func (p *PerformanceObligation) Create(w http.ResponseWriter, r *http.Request) {

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
		func() createPerformanceObligation { return createPerformanceObligation{} },
		func(req createPerformanceObligation) (database.CreatePerformanceObligationParams, error) {
			dbReq := database.CreatePerformanceObligationParams{
				PerformanceObligationsName: req.PerformanceObligationsName,
				StartDate:                  req.StartDate,
				EndDate:                    req.EndDate,
				FunctionalCurrency:         req.FunctionalCurrency,
				Discount:                   req.Discount,
				TransactionPrice:           req.TransactionPrice,
			}

			payload := performanceObPayload{
				PerformanceObligationsName: dbReq.PerformanceObligationsName,
				StartDate:                  dbReq.StartDate,
				EndDate:                    dbReq.EndDate,
				FunctionalCurrency:         dbReq.FunctionalCurrency,
				Discount:                   dbReq.Discount,
				TransactionPrice:           dbReq.TransactionPrice,
			}

			if err := validatePerformanceObStrict(&payload); err != nil {
				return dbReq, err
			}

			dbReq.ContractID = contractID
			dbReq.PerformanceObligationsName = payload.PerformanceObligationsName
			dbReq.StartDate = payload.StartDate
			dbReq.EndDate = payload.EndDate
			dbReq.FunctionalCurrency = payload.FunctionalCurrency
			dbReq.Discount = payload.Discount
			dbReq.TransactionPrice = payload.TransactionPrice

			return dbReq, nil
		},
		func(ctx context.Context, params database.CreatePerformanceObligationParams) (database.PerformanceObligation, error) {
			return p.DB.CreatePerformanceObligation(ctx, params)
		},
		http.StatusCreated,
	)

}

func (p *PerformanceObligation) List(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, params uuid.UUID) ([]database.PerformanceObligation, error) {
			return p.DB.GetPerformanceObligationsForCompany(ctx, params)
		},
		http.StatusOK,
	)

}

func (p *PerformanceObligation) ListAll(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, params interface{}) ([]database.PerformanceObligation, error) {
			return p.DB.GetAllPerformanceObligations(ctx)
		},
		http.StatusOK,
	)

}

func (p *PerformanceObligation) GetById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	performanceObID, err := uuid.Parse(chi.URLParam(r, "performanceObID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid performance obligation ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetPerformanceObligationParams, error) {
			return database.GetPerformanceObligationParams{
				ID:        performanceObID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, params database.GetPerformanceObligationParams) (database.PerformanceObligation, error) {
			return p.DB.GetPerformanceObligation(ctx, params)
		},
		http.StatusOK,
	)

}

func (p *PerformanceObligation) GetForContract(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
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
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetPerformanceObligationsForContractParams, error) {
			return database.GetPerformanceObligationsForContractParams{
				ContractID: contractID,
				CompanyID:  companyID,
			}, nil
		},
		func(ctx context.Context, params database.GetPerformanceObligationsForContractParams) ([]database.PerformanceObligation, error) {
			return p.DB.GetPerformanceObligationsForContract(ctx, params)
		},
		http.StatusOK,
	)

}

func (p *PerformanceObligation) UpdateById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	contractID, err := uuid.Parse(chi.URLParam(r, "contractID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid contract ID:", err)
		return
	}

	performanceObID, err := uuid.Parse(chi.URLParam(r, "performanceObID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid performance obligation ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() updatePerformanceObligation { return updatePerformanceObligation{} },
		func(req updatePerformanceObligation) (database.UpdatePerformanceObligationParams, error) {
			dbReq := database.UpdatePerformanceObligationParams{
				PerformanceObligationsName: req.PerformanceObligationsName,
				StartDate:                  req.StartDate,
				EndDate:                    req.EndDate,
				FunctionalCurrency:         req.FunctionalCurrency,
				Discount:                   req.Discount,
				TransactionPrice:           req.TransactionPrice,
			}

			payload := performanceObPayload{
				PerformanceObligationsName: dbReq.PerformanceObligationsName,
				StartDate:                  dbReq.StartDate,
				EndDate:                    dbReq.EndDate,
				FunctionalCurrency:         dbReq.FunctionalCurrency,
				Discount:                   dbReq.Discount,
				TransactionPrice:           dbReq.TransactionPrice,
			}

			if err := validatePerformanceObStrict(&payload); err != nil {
				return dbReq, err
			}

			dbReq.PerformanceObligationsName = payload.PerformanceObligationsName
			dbReq.ContractID = contractID
			dbReq.StartDate = payload.StartDate
			dbReq.EndDate = payload.EndDate
			dbReq.FunctionalCurrency = payload.FunctionalCurrency
			dbReq.Discount = payload.Discount
			dbReq.TransactionPrice = payload.TransactionPrice
			dbReq.ID = performanceObID
			dbReq.CompanyID = companyID

			return dbReq, nil
		},
		func(ctx context.Context, params database.UpdatePerformanceObligationParams) (database.PerformanceObligation, error) {
			return p.DB.UpdatePerformanceObligation(ctx, params)
		},
		http.StatusOK,
	)

}

func (p *PerformanceObligation) DeleteById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	performanceObID, err := uuid.Parse(chi.URLParam(r, "performanceObID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid performance obligation ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.DeletePerformanceObligationParams, error) {
			return database.DeletePerformanceObligationParams{
				ID:        performanceObID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, params database.DeletePerformanceObligationParams) (interface{}, error) {
			return struct{}{}, p.DB.DeletePerformanceObligation(ctx, params)
		},
		http.StatusOK,
	)
}

func (p *PerformanceObligation) ResetTable(w http.ResponseWriter, r *http.Request) {
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
			return struct{}{}, p.DB.ResetPerformanceObligations(ctx)
		},
		http.StatusOK,
	)
}

func (b *Bundle) AddProdToPerformOb(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	performanceObID, err := uuid.Parse(chi.URLParam(r, "performanceObID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid performance obligation ID:", err)
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
		func(req interface{}) (database.AddProductToPerformanceObligationParams, error) {
			return database.AddProductToPerformanceObligationParams{
				ID_2:      performanceObID,
				ID:        productID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.AddProductToPerformanceObligationParams) (database.ProductPerformanceObligation, error) {
			return b.DB.AddProductToPerformanceObligation(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) DeleteProdToPerformOb(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	performanceObID, err := uuid.Parse(chi.URLParam(r, "performanceObID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid performance obligation ID:", err)
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
		func(req interface{}) (database.DeleteProductFromPerformanceObligationParams, error) {
			return database.DeleteProductFromPerformanceObligationParams{
				PerformanceObligationsID: performanceObID,
				ProductID:                productID,
				CompanyID:                companyID,
			}, nil
		},
		func(ctx context.Context, param database.DeleteProductFromPerformanceObligationParams) (interface{}, error) {
			return struct{}{}, b.DB.DeleteProductFromPerformanceObligation(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) GetProdsInPerformOb(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	performanceObID, err := uuid.Parse(chi.URLParam(r, "performanceObID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid performance obligation ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetPerformanceObligationProductsParams, error) {
			return database.GetPerformanceObligationProductsParams{
				PerformanceObligationsID: performanceObID,
				CompanyID:                companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetPerformanceObligationProductsParams) ([]database.Product, error) {
			return b.DB.GetPerformanceObligationProducts(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) GetPerformObInProds(w http.ResponseWriter, r *http.Request) {

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
		func(req interface{}) (database.GetPerformanceObligationsForProductParams, error) {
			return database.GetPerformanceObligationsForProductParams{
				ProductID: productID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetPerformanceObligationsForProductParams) ([]database.PerformanceObligation, error) {
			return b.DB.GetPerformanceObligationsForProduct(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) ClearProdsFromPerformOb(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	performanceObID, err := uuid.Parse(chi.URLParam(r, "performanceObID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid performance obligation ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.ClearPerformanceObligationProductsParams, error) {
			return database.ClearPerformanceObligationProductsParams{
				PerformanceObligationsID: performanceObID,
				CompanyID:                companyID,
			}, nil
		},
		func(ctx context.Context, param database.ClearPerformanceObligationProductsParams) (struct{}, error) {
			return struct{}{}, b.DB.ClearPerformanceObligationProducts(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) ResetTableProdPerformOb(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (struct{}, error) {
			return struct{}{}, nil
		},
		func(ctx context.Context, param struct{}) (struct{}, error) {
			return struct{}{}, b.DB.ResetProductPerformanceObligations(ctx)
		},
		http.StatusOK,
	)

}

/////

func (b *Bundle) AddBunToPerformOb(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	performanceObID, err := uuid.Parse(chi.URLParam(r, "performanceObID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid performance obligation ID:", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.AddBundleToPerformanceObligationParams, error) {
			return database.AddBundleToPerformanceObligationParams{
				ID_2:      performanceObID,
				ID:        bundleID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.AddBundleToPerformanceObligationParams) (database.BundlePerformanceObligation, error) {
			return b.DB.AddBundleToPerformanceObligation(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) DeleteBunToPerformOb(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	performanceObID, err := uuid.Parse(chi.URLParam(r, "performanceObID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid performance obligation ID:", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.DeleteBundleFromPerformanceObligationParams, error) {
			return database.DeleteBundleFromPerformanceObligationParams{
				PerformanceObligationsID: performanceObID,
				BundleID:                 bundleID,
				CompanyID:                companyID,
			}, nil
		},
		func(ctx context.Context, param database.DeleteBundleFromPerformanceObligationParams) (interface{}, error) {
			return struct{}{}, b.DB.DeleteBundleFromPerformanceObligation(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) GetBunsInPerformOb(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	performanceObID, err := uuid.Parse(chi.URLParam(r, "performanceObID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid performance obligation ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetPerformanceObligationBundlesParams, error) {
			return database.GetPerformanceObligationBundlesParams{
				PerformanceObligationsID: performanceObID,
				CompanyID:                companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetPerformanceObligationBundlesParams) ([]database.Bundle, error) {
			return b.DB.GetPerformanceObligationBundles(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) GetPerformObInBuns(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetPerformanceObligationsForBundleParams, error) {
			return database.GetPerformanceObligationsForBundleParams{
				BundleID:  bundleID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetPerformanceObligationsForBundleParams) ([]database.PerformanceObligation, error) {
			return b.DB.GetPerformanceObligationsForBundle(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) ClearBunsFromPerformOb(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID:", err)
		return
	}

	performanceObID, err := uuid.Parse(chi.URLParam(r, "performanceObID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid performance obligation ID:", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.ClearPerformanceObligationBundlesParams, error) {
			return database.ClearPerformanceObligationBundlesParams{
				PerformanceObligationsID: performanceObID,
				CompanyID:                companyID,
			}, nil
		},
		func(ctx context.Context, param database.ClearPerformanceObligationBundlesParams) (struct{}, error) {
			return struct{}{}, b.DB.ClearPerformanceObligationBundles(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) ResetTableBunPerformOb(w http.ResponseWriter, r *http.Request) {

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (struct{}, error) {
			return struct{}{}, nil
		},
		func(ctx context.Context, param struct{}) (struct{}, error) {
			return struct{}{}, b.DB.ResetBundlePerformanceObligations(ctx)
		},
		http.StatusOK,
	)

}
