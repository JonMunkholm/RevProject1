package main

import (
	"time"

	"github.com/google/uuid"
)



type Customer struct {
	ID 				uuid.UUID
    CustomerName 	string
    CreatedAt 		time.Time
    UpdatedAt 		time.Time
    IsActive 		bool
    CompanyID 		uuid.UUID
}

type Product struct {
	ID 								uuid.UUID
    ProdName 						string
    RevAssessment 					*string
    OverTimePercent 				*float64
    PointInTimePercent 				*float64
    StandaloneSellingPriceMethod 	*string
    StandaloneSellingPricePrice  	*float64
    CompanyID 						uuid.UUID
    IsActive  						bool
    DefaultCurrency 				string
    CreatedAt  						time.Time
}

type Bundle struct {
	ID 				uuid.UUID
    BundleName 	string
    CompanyID 		uuid.UUID
    IsActive 		bool
	Products   		*[]Product
}

type PerformanceObligation struct {
	ID 								uuid.UUID
	PerformanceObligationsName 		string
    ContractID 						uuid.UUID
    CreatedAt 						time.Time
    UpdatedAt 						time.Time
    StartDate 						time.Time
    EndDate 						time.Time
    FunctionalCurrency 				string
    Discount 						float64
    TransactionPrice 				float64
    IsFinal 						bool
	Bundles							*[]Bundle
	Products						*[]Product
}

type Contract struct {
	ID 							uuid.UUID
    CompanyID 					uuid.UUID
    CustomerID 					uuid.UUID
    CreatedAt 					time.Time
    UpdatedAt 					time.Time
    StartDate 					time.Time
    EndDate 					time.Time
    IsFinal 					bool
    ContractURL 				string
	PerformanceObligations 		*[]PerformanceObligation
}
