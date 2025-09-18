package application

import (
	"net/http"

	"github.com/JonMunkholm/RevProject1/internal/handler"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func (a *App) loadRoutes () {
	r := chi.NewRouter()

	r.Use(middleware.Logger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Root is being served"))
	})

	r.Route("/users", a.loadUserRoutes)
	r.Route("/companies", a.loadCompanyRoutes)
	r.Route("/customers", a.loadCustomerRoutes)
	r.Route("/admin", a.loadAdminRoutes)



	a.router = r
}

func (a *App) loadUserRoutes (r chi.Router) {
	//allows for additional routs to be added easier
	userHandler := &handler.User{
		DB: a.db,
	}

	r.Post("/", userHandler.Create)
	r.Get("/", userHandler.List)
	r.Get("/{id}", userHandler.GetById)
	r.Put("/{id}", userHandler.UpdateById)
	r.Delete("/{id}", userHandler.DeleteById)
}

func (a *App) loadCompanyRoutes (r chi.Router) {
	//allows for additional routs to be added easier
	companyHandler := &handler.Company{
		DB: a.db,
	}

	r.Post("/", companyHandler.Create)
	r.Get("/", companyHandler.List)
	r.Get("/{id}", companyHandler.GetById)
	r.Put("/{id}", companyHandler.UpdateById)
	r.Delete("/{id}", companyHandler.DeleteById)
}

func (a *App) loadCustomerRoutes (r chi.Router) {
	//allows for additional routs to be added easier
	companyHandler := &handler.Customer{
		DB: a.db,
	}

	r.Post("/", companyHandler.Create)
	r.Get("/", companyHandler.List)
	r.Get("/{id}", companyHandler.GetById)
	r.Put("/{id}", companyHandler.UpdateById)
	r.Delete("/{id}", companyHandler.DeleteById)
}




func (a *App) loadAdminRoutes (r chi.Router) {
	//allows for additional routs to be added easier
	adminHandler := &handler.Admin{
		DB: a.db,
	}

	r.Post("/quickStart", adminHandler.QuickStart)
	r.Delete("/reset", adminHandler.Reset)
}
