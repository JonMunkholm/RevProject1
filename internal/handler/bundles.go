package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type Bundle struct {
	DB *database.Queries
}

type bundleParam struct {
	BundleName string `json:"BundleName"`
}

type updateBundle struct {
	BundleName string `json:"BundleName"`
	IsActive   bool   `json:"IsActive"`
}

func (b *Bundle) Create(w http.ResponseWriter, r *http.Request) {

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
		func() bundleParam { return bundleParam{} },
		func(req bundleParam) (database.CreateBundleParams, error) {
			name := strings.TrimSpace(req.BundleName)
			if name == "" {
				return database.CreateBundleParams{}, fmt.Errorf("BundleName is required")
			}
			return database.CreateBundleParams{
				BundleName: name,
				CompanyID:  companyID,
			}, nil
		},
		func(ctx context.Context, params database.CreateBundleParams) (database.Bundle, error) {
			return b.DB.CreateBundle(ctx, params)
		},
		http.StatusCreated,
	)

}

func (b *Bundle) DeleteByID(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.DeleteBundleParams, error) {
			return database.DeleteBundleParams{
				ID:        bundleID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, params database.DeleteBundleParams) (struct{}, error) {
			return struct{}{}, b.DB.DeleteBundle(ctx, params)
		},
		http.StatusOK,
	)

}

func (b *Bundle) UpdateById(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() updateBundle { return updateBundle{} },
		func(req updateBundle) (database.UpdateBundleParams, error) {
			name := strings.TrimSpace(req.BundleName)
			if name == "" {
				return database.UpdateBundleParams{}, fmt.Errorf("BundleName is required")
			}
			return database.UpdateBundleParams{
				BundleName: name,
				IsActive:   req.IsActive,
				ID:         bundleID,
				CompanyID:  companyID,
			}, nil
		},
		func(ctx context.Context, params database.UpdateBundleParams) (database.Bundle, error) {
			return b.DB.UpdateBundle(ctx, params)
		},
		http.StatusOK,
	)

}

func (b *Bundle) GetByID(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetBundleParams, error) {
			return database.GetBundleParams{
				ID:        bundleID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, params database.GetBundleParams) (database.Bundle, error) {
			return b.DB.GetBundle(ctx, params)
		},
		http.StatusOK,
	)

}

func (b *Bundle) GetByName(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	bundleName := strings.TrimSpace(chi.URLParam(r, "bundleName"))
	if bundleName == "" {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle name", fmt.Errorf("bundleName is required"))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetBundleByNameParams, error) {
			return database.GetBundleByNameParams{
				BundleName: bundleName,
				CompanyID:  companyID,
			}, nil
		},
		func(ctx context.Context, params database.GetBundleByNameParams) (database.Bundle, error) {
			return b.DB.GetBundleByName(ctx, params)
		},
		http.StatusOK,
	)

}

func (b *Bundle) SetBundleActiveStatus(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() setActiveRequest { return setActiveRequest{} },
		func(req setActiveRequest) (database.SetBundleActiveStatusParams, error) {
			return database.SetBundleActiveStatusParams{
				IsActive:  req.IsActive,
				ID:        bundleID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, params database.SetBundleActiveStatusParams) (struct{}, error) {
			return struct{}{}, b.DB.SetBundleActiveStatus(ctx, params)
		},
		http.StatusOK,
	)

}

func (b *Bundle) List(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, param uuid.UUID) ([]database.Bundle, error) {
			return b.DB.GetAllBundlesCompany(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) GetActive(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, param uuid.UUID) ([]database.Bundle, error) {
			return b.DB.GetActiveBundlesCompany(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) ListAll(w http.ResponseWriter, r *http.Request) {

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
		func(ctx context.Context, param struct{}) ([]database.Bundle, error) {
			return b.DB.GetAllBundles(ctx)
		},
		http.StatusOK,
	)

}

func (b *Bundle) ResetTableBun(w http.ResponseWriter, r *http.Request) {

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
			return struct{}{}, b.DB.ResetBundles(ctx)
		},
		http.StatusOK,
	)

}

func (b *Bundle) AddProdToBun(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID", err)
		return
	}

	productID, err := uuid.Parse(chi.URLParam(r, "productID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid product ID", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.AddProductToBundleParams, error) {
			return database.AddProductToBundleParams{
				BundleID:  bundleID,
				ProductID: productID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.AddProductToBundleParams) (database.BundleProduct, error) {
			return b.DB.AddProductToBundle(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) DeleteProdFromBun(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID", err)
		return
	}

	productID, err := uuid.Parse(chi.URLParam(r, "productID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid product ID", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.DeleteProductFromBundleParams, error) {
			return database.DeleteProductFromBundleParams{
				BundleID:  bundleID,
				ProductID: productID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.DeleteProductFromBundleParams) (struct{}, error) {
			return struct{}{}, b.DB.DeleteProductFromBundle(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) GetProdsInBun(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetBundleProductsParams, error) {
			return database.GetBundleProductsParams{
				BundleID:  bundleID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetBundleProductsParams) ([]database.BundleProduct, error) {
			return b.DB.GetBundleProducts(ctx, param)
		},
		http.StatusOK,
	)

}

// list the all products in a bundle w/ the product details
func (b *Bundle) GetProdsInBunDetail(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetBundleProductDetailsParams, error) {
			return database.GetBundleProductDetailsParams{
				BundleID:  bundleID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetBundleProductDetailsParams) ([]database.GetBundleProductDetailsRow, error) {
			return b.DB.GetBundleProductDetails(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) ClearProdsFromBun(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	bundleID, err := uuid.Parse(chi.URLParam(r, "bundleID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid bundle ID", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.ClearBundleProductsParams, error) {
			return database.ClearBundleProductsParams{
				BundleID:  bundleID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.ClearBundleProductsParams) (struct{}, error) {
			return struct{}{}, b.DB.ClearBundleProducts(ctx, param)
		},
		http.StatusOK,
	)

}

func (b *Bundle) ResetTableProdBun(w http.ResponseWriter, r *http.Request) {

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
			return struct{}{}, b.DB.ResetBundleProducts(ctx)
		},
		http.StatusOK,
	)

}

func (b *Bundle) GetBunsWithProd(w http.ResponseWriter, r *http.Request) {

	companyID, err := uuid.Parse(chi.URLParam(r, "companyID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid company ID", err)
		return
	}

	productID, err := uuid.Parse(chi.URLParam(r, "productID"))
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Error missing or invalid product ID", err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	_, _ = processRequest(
		ctx,
		w,
		r,
		func() interface{} { return struct{}{} },
		func(req interface{}) (database.GetBundlesForProductParams, error) {
			return database.GetBundlesForProductParams{
				ProductID: productID,
				CompanyID: companyID,
			}, nil
		},
		func(ctx context.Context, param database.GetBundlesForProductParams) ([]database.Bundle, error) {
			return b.DB.GetBundlesForProduct(ctx, param)
		},
		http.StatusOK,
	)

}
