package handler

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/google/uuid"
)

func (c *Contract) contractInputValidation(ct *database.CreateContractParams) error {
	if ct == nil {
		return fmt.Errorf("missing contract payload")
	}

	payload := contractPayload{
		CompanyID:   ct.CompanyID,
		CustomerID:  ct.CustomerID,
		StartDate:   ct.StartDate,
		EndDate:     ct.EndDate,
		ContractURL: ct.ContractUrl,
	}

	if err := validateContractStrict(&payload); err != nil {
		return err
	}

	ct.ContractUrl = payload.ContractURL

	return nil
}

func (c *Contract) contractUpdateValidation(ct *database.UpdateContractParams) error {
	if ct == nil {
		return fmt.Errorf("missing contract payload")
	}

	payload := contractPayload{
		CompanyID:   ct.CompanyID,
		CustomerID:  ct.CustomerID,
		StartDate:   ct.StartDate,
		EndDate:     ct.EndDate,
		ContractURL: ct.ContractUrl,
	}

	if err := validateContractStrict(&payload); err != nil {
		return err
	}

	ct.ContractUrl = payload.ContractURL

	return nil
}

type contractPayload struct {
	CompanyID   uuid.UUID
	CustomerID  uuid.UUID
	StartDate   time.Time
	EndDate     time.Time
	ContractURL sql.NullString
}

type contractValidator func(*contractPayload) error

func validateContractStrict(p *contractPayload) error {
	return runContractValidators(p,
		requireContractCompanyID(),
		requireContractCustomerID(),
		requireContractDates(),
		normalizeContractURL(),
	)
}

func runContractValidators(p *contractPayload, validators ...contractValidator) error {
	for _, v := range validators {
		if err := v(p); err != nil {
			return err
		}
	}
	return nil
}

func requireContractCompanyID() contractValidator {
	return func(p *contractPayload) error {
		if p.CompanyID == uuid.Nil {
			return fmt.Errorf("CompanyID is required")
		}
		return nil
	}
}

func requireContractCustomerID() contractValidator {
	return func(p *contractPayload) error {
		if p.CustomerID == uuid.Nil {
			return fmt.Errorf("CustomerID is required")
		}
		return nil
	}
}

func requireContractDates() contractValidator {
	return func(p *contractPayload) error {
		if p.StartDate.IsZero() {
			return fmt.Errorf("StartDate is required")
		}
		if p.EndDate.IsZero() {
			return fmt.Errorf("EndDate is required")
		}
		if !p.EndDate.After(p.StartDate) {
			return fmt.Errorf("EndDate must be after StartDate")
		}
		return nil
	}
}

func normalizeContractURL() contractValidator {
	return func(p *contractPayload) error {
		if !p.ContractURL.Valid {
			return nil
		}

		trimmed := strings.TrimSpace(p.ContractURL.String)
		if trimmed == "" {
			p.ContractURL = sql.NullString{}
			return nil
		}

		p.ContractURL = sql.NullString{String: trimmed, Valid: true}
		return nil
	}
}

// productPayload represents the product fields that share validation logic
// across different handlers. Keeping it separate from sqlc structs makes it
// easy to reuse while still benefiting from compile-time checks.
type productPayload struct {
	CompanyID uuid.UUID

	ProdName           string
	RevAssessment      string
	OverTimePercent    string
	PointInTimePercent string

	SSPMethod string
	SSPHigh   string
	SSPLow    string

	DefaultCurrency string
}

// payloadValidator is a function that validates an aspect of productPayload.
type payloadValidator func(*productPayload) error

// validateProductStrict runs the full set of product validations.
func validateProductStrict(p *productPayload) error {
	return runProductValidators(p,
		requireCompanyID(),
		requireName(),
		requireAssessment(),
		requirePercents(),
		requireSSPBounds(),
		requireCurrency(),
	)
}

// runProductValidators executes the provided validators in order, returning
// the first error encountered.
func runProductValidators(p *productPayload, validators ...payloadValidator) error {
	for _, v := range validators {
		if err := v(p); err != nil {
			return err
		}
	}
	return nil
}

func requireCompanyID() payloadValidator {
	return func(p *productPayload) error {
		if p.CompanyID == uuid.Nil {
			return fmt.Errorf("CompanyID is required")
		}
		return nil
	}
}

func requireName() payloadValidator {
	return func(p *productPayload) error {
		name := strings.TrimSpace(p.ProdName)
		if name == "" {
			return fmt.Errorf("ProdName is required")
		}
		if len(name) > 255 {
			return fmt.Errorf("ProdName exceeds 255 characters")
		}
		p.ProdName = name
		return nil
	}
}

func requireAssessment() payloadValidator {
	allowed := map[string]struct{}{
		"over_time":     {},
		"point_in_time": {},
		"split":         {},
	}
	return func(p *productPayload) error {
		if _, ok := allowed[p.RevAssessment]; !ok {
			return fmt.Errorf("RevAssessment must be over_time, point_in_time, or split")
		}
		return nil
	}
}

func requirePercents() payloadValidator {
	return func(p *productPayload) error {
		over, err := parsePercent("OverTimePercent", p.OverTimePercent)
		if err != nil {
			return err
		}
		pit, err := parsePercent("PointInTimePercent", p.PointInTimePercent)
		if err != nil {
			return err
		}

		switch p.RevAssessment {
		case "over_time":
			if !floatEquals(over, 1) || !floatEquals(pit, 0) {
				return fmt.Errorf("OverTimePercent must be 1.0000 and PointInTimePercent 0.0000 for over_time products")
			}
		case "point_in_time":
			if !floatEquals(pit, 1) || !floatEquals(over, 0) {
				return fmt.Errorf("PointInTimePercent must be 1.0000 and OverTimePercent 0.0000 for point_in_time products")
			}
		case "split":
			if math.Abs(over+pit-1) > 1e-6 {
				return fmt.Errorf("OverTimePercent plus PointInTimePercent must equal 1.0000 for split products")
			}
		}
		return nil
	}
}

func requireSSPBounds() payloadValidator {
	allowed := map[string]struct{}{
		"observable":      {},
		"adjusted_market": {},
		"cost_plus":       {},
		"residual":        {},
	}
	return func(p *productPayload) error {
		if _, ok := allowed[p.SSPMethod]; !ok {
			return fmt.Errorf("StandaloneSellingPriceMethod must be observable, adjusted_market, cost_plus, or residual")
		}

		high, err := parsePrice("StandaloneSellingPricePriceHigh", p.SSPHigh)
		if err != nil {
			return err
		}
		low, err := parsePrice("StandaloneSellingPricePriceLow", p.SSPLow)
		if err != nil {
			return err
		}
		if high < low {
			return fmt.Errorf("StandaloneSellingPricePriceHigh must be greater than or equal to StandaloneSellingPricePriceLow")
		}
		return nil
	}
}

func requireCurrency() payloadValidator {
	return func(p *productPayload) error {
		code := strings.ToUpper(strings.TrimSpace(p.DefaultCurrency))
		if len(code) != 3 {
			return fmt.Errorf("DefaultCurrency must be a 3-letter ISO code")
		}
		p.DefaultCurrency = code
		return nil
	}
}

func parsePercent(label, value string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a numeric value", label)
	}
	if parsed < 0 || parsed > 1 {
		return 0, fmt.Errorf("%s must be between 0.0000 and 1.0000", label)
	}
	return parsed, nil
}

func parsePrice(label, value string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a numeric value", label)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must be greater than or equal to zero", label)
	}
	return parsed, nil
}

func floatEquals(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9
}
