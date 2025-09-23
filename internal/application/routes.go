package application

import (
	"net/http"

	"github.com/JonMunkholm/RevProject1/internal/handler"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func (a *App) loadRoutes() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Root is being served"))
	})

	r.Route("/companies", a.loadCompanyRoutes)

	r.Route("/admin", a.loadAdminRoutes)

	a.router = r
}

func (a *App) loadCompanyRoutes(r chi.Router) {
	//allows for additional routs to be added easier
	//most all these endpoints will be moved to owner route
	companyHandler := &handler.Company{
		DB: a.db,
	}

	r.Post("/", companyHandler.Create)
	r.Get("/", companyHandler.List)

	r.Get("/active", companyHandler.GetActive)
	r.Get("/by-name/{name}", companyHandler.GetByName)

	r.Put("/{companyID}/active", companyHandler.SetActive)
	r.Get("/{companyID}", companyHandler.GetById)
	r.Put("/{companyID}", companyHandler.UpdateById)
	r.Delete("/{companyID}", companyHandler.DeleteById)

	r.Route("/{companyID}/users", a.loadUserRoutes)
	r.Route("/{companyID}/customers", a.loadCustomerRoutes)
	r.Route("/{companyID}/products", a.loadProductRoutes)
	r.Route("/{companyID}/contracts", a.loadContractRoutes)

}

func (a *App) loadUserRoutes(r chi.Router) {
	userHandler := &handler.User{DB: a.db}

	r.Post("/", userHandler.Create)
	r.Get("/", userHandler.List)
	r.Get("/active", userHandler.GetActive)
	r.Get("/by-name/{name}", userHandler.GetByName)

	r.Get("/{userID}", userHandler.GetById)
	r.Put("/{userID}", userHandler.UpdateById)
	r.Put("/{userID}/active", userHandler.SetActive)
	r.Delete("/{userID}", userHandler.DeleteById)

}

func (a *App) loadCustomerRoutes(r chi.Router) {
	customerHandler := &handler.Customer{DB: a.db}

	r.Post("/", customerHandler.Create)
	r.Get("/", customerHandler.List)
	r.Get("/active", customerHandler.GetActive)
	r.Get("/by-name/{name}", customerHandler.GetByName)

	r.Get("/{customerID}", customerHandler.GetById)
	r.Put("/{customerID}", customerHandler.UpdateById)
	r.Put("/{customerID}/active", customerHandler.SetActive)
	r.Delete("/{customerID}", customerHandler.DeleteById)

}

func (a *App) loadContractRoutes(r chi.Router) {
	//allows for additional routs to be added easier
	contractHandler := &handler.Contract{
		DB: a.db,
	}

	r.Post("/", contractHandler.Create)
	r.Get("/", contractHandler.List)
	r.Get("/final", contractHandler.GetFinal)
	r.Get("/customers/{customerID}", contractHandler.ListCustomer)

	r.Get("/{contractID}", contractHandler.GetById)
	r.Put("/{contractID}", contractHandler.UpdateById)
	r.Delete("/{contractID}", contractHandler.DeleteById)

}

func (a *App) loadProductRoutes(r chi.Router) {
	productHandler := &handler.Product{DB: a.db}

	r.Post("/", productHandler.Create)
	r.Get("/", productHandler.List)
	r.Get("/active", productHandler.GetActive)
	r.Get("/by-name/{productName}", productHandler.GetByName)

	r.Get("/{productID}", productHandler.GetById)
	r.Put("/{productID}", productHandler.UpdateById)
	r.Put("/{productID}/active", productHandler.SetActive)
	r.Delete("/{productID}", productHandler.DeleteById)
}

func (a *App) loadAdminRoutes(r chi.Router) {
	adminHandler := &handler.Admin{DB: a.db}
	r.Post("/quickStart", adminHandler.QuickStart)
	r.Delete("/reset", adminHandler.Reset)

	companyHandler := &handler.Company{DB: a.db}
	r.Delete("/companies", companyHandler.ResetDB)

	userHandler := &handler.User{DB: a.db}
	r.Get("/users", userHandler.ListAll)
	r.Delete("/users", userHandler.ResetTable)

	customerHandler := &handler.Customer{DB: a.db}
	r.Get("/customers", customerHandler.ListAll)
	r.Delete("/customers", customerHandler.ResetTable)

	contractHandler := &handler.Contract{DB: a.db}
	r.Get("/contracts", contractHandler.ListAll)
	r.Delete("/contracts", contractHandler.ResetTable)

	productHandler := &handler.Product{DB: a.db}
	r.Get("/products", productHandler.ListAll)
}
