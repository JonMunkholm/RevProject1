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
	r.Route("/products", a.loadProductRoutes)
	r.Route("/contracts", a.loadContractRoutes)

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

	r.Delete("/", companyHandler.ResetDB)

	r.Route("/{companyID}/users", a.loadUserRoutes)
	r.Route("/{companyID}/customers", a.loadCustomerRoutes)

}

func (a *App) loadUserRoutes(r chi.Router) {
	//allows for additional routs to be added easier
	userHandler := &handler.User{
		DB: a.db,
	}

	r.Post("/", userHandler.Create)
	r.Get("/", userHandler.ListAll) //move to owner route and rename function for handler below and function in user.go to List
	r.Get("/company", userHandler.List)

	r.Get("/by-name/{name}", userHandler.GetByName)

	r.Get("/{userID}", userHandler.GetById)
	r.Get("/{userID}/active", userHandler.GetActive)
	r.Put("/{userID}/active", userHandler.SetActive)

	r.Put("/{userID}", userHandler.UpdateById)
	r.Delete("/{userID}", userHandler.DeleteById)

	r.Delete("/", userHandler.ResetTable) //move to owner route

}

func (a *App) loadCustomerRoutes(r chi.Router) {
	//allows for additional routs to be added easier
	customerHandler := &handler.Customer{
		DB: a.db,
	}

	r.Post("/", customerHandler.Create)
	r.Get("/", customerHandler.ListAll) //move to owner route and rename function for handler below and function in user.go to List
	r.Get("/company", customerHandler.List)

	r.Get("/by-name/{name}", customerHandler.GetByName)

	r.Get("/{customerID}", customerHandler.GetById)
	r.Get("/{customerID}/active", customerHandler.GetActive)
	r.Put("/{customerID}/active", customerHandler.SetActive)

	r.Put("/{customerID}", customerHandler.UpdateById)
	r.Delete("/{customerID}", customerHandler.DeleteById)

	r.Delete("/", customerHandler.ResetTable) //move to owner route

}

func (a *App) loadProductRoutes(r chi.Router) {
	//allows for additional routs to be added easier
	productHandler := &handler.Product{
		DB: a.db,
	}

	r.Post("/", productHandler.Create)
	r.Get("/", productHandler.List)
	r.Get("/{id}", productHandler.GetById)
	r.Put("/{id}", productHandler.UpdateById)
	r.Delete("/{id}", productHandler.DeleteById)
}

func (a *App) loadContractRoutes(r chi.Router) {
	//allows for additional routs to be added easier
	contractHandler := &handler.Contract{
		DB: a.db,
	}

	r.Post("/", contractHandler.Create)
	r.Get("/", contractHandler.List)
	r.Get("/{id}", contractHandler.GetById)
	r.Put("/{id}", contractHandler.UpdateById)
	r.Delete("/{id}", contractHandler.DeleteById)
}

func (a *App) loadAdminRoutes(r chi.Router) {
	//allows for additional routs to be added easier
	adminHandler := &handler.Admin{
		DB: a.db,
	}

	r.Post("/quickStart", adminHandler.QuickStart)
	r.Delete("/reset", adminHandler.Reset)
}
