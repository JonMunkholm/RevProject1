package application

import (
	"net/http"

	appviews "github.com/JonMunkholm/RevProject1/app"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/handler"
	"github.com/a-h/templ"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

func (a *App) loadRoutes() {
	r := chi.NewRouter()

	r.Use(middleware.Logger)

	serveAppAssets(r)

	// Public landing + login flow
	r.Group(func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			a.render(w, r, appviews.LandingPage())
		})

		r.Route("/login", a.loadLogin)
		r.Route("/register", a.loadRegister)
		r.Route("/auth", a.loadAuthRoutes)
	})

	// Authenticated application + API surface
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(a.jwtSecret))

		r.Route("/app", func(r chi.Router) {
			serveAppShell := func(w http.ResponseWriter, r *http.Request) {
				http.ServeFile(w, r, "app/index.html")
			}

			for _, route := range []string{
				"/",
				"/dashboard",
				"/settings",
				"/companies",
				"/customers",
				"/contracts",
				"/products",
				"/performance-obligations",
				"/bundles",
			} {
				r.Get(route, serveAppShell)
			}
		})

		r.Route("/api", func(r chi.Router) {
			r.Route("/companies", a.loadCompanyRoutes)
			r.Route("/admin", a.loadAdminRoutes)
		})
	})

	a.router = r
}

func serveAppAssets(r chi.Router) {
	static := http.FileServer(http.Dir("app"))
	r.Handle("/styles.css", static)
	r.Handle("/login.html", static)
	r.Handle("/fonts/*", http.StripPrefix("/fonts/", http.FileServer(http.Dir("app/fonts"))))
}

func (a *App) render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func (a *App) loadLogin(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		a.render(w, r, appviews.LoginPage())
	})
}

func (a *App) loadRegister(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "app/register.html")
	})
}

func (a *App) loadAuthRoutes(r chi.Router) {
	loginHandler := &auth.Login{
		DB:        a.db,
		JWTSecret: a.jwtSecret,
	}

	r.Post("/login", loginHandler.SignIn)
	r.Post("/register", loginHandler.Register)
	r.Post("/refresh", loginHandler.Refresh)
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
	r.Route("/{companyID}/performance-obligations", a.loadPerformanceObRoutes)
	r.Route("/{companyID}/bundles", a.loadBundleRoutes)

}

func (a *App) loadUserRoutes(r chi.Router) {
	userHandler := &handler.User{DB: a.db}

	r.Post("/", userHandler.Create)
	r.Get("/", userHandler.List)
	r.Get("/active", userHandler.GetActive)
	r.Get("/by-email/{email}", userHandler.GetByEmail)

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
	performanceObHandler := &handler.PerformanceObligation{DB: a.db}

	r.Post("/", contractHandler.Create)
	r.Get("/", contractHandler.List)
	r.Get("/final", contractHandler.GetFinal)
	r.Get("/customers/{customerID}", contractHandler.ListCustomer)

	r.Get("/{contractID}", contractHandler.GetById)
	r.Put("/{contractID}", contractHandler.UpdateById)
	r.Delete("/{contractID}", contractHandler.DeleteById)

	r.Route("/{contractID}/performance-obligations", func(r chi.Router) {
		r.Post("/", performanceObHandler.Create)
		r.Get("/", performanceObHandler.GetForContract)
		r.Get("/{performanceObID}", performanceObHandler.GetById)
		r.Put("/{performanceObID}", performanceObHandler.UpdateById)
		r.Delete("/{performanceObID}", performanceObHandler.DeleteById)
	})

}

func (a *App) loadProductRoutes(r chi.Router) {
	productHandler := &handler.Product{DB: a.db}
	bundleHandler := &handler.Bundle{DB: a.db}

	r.Post("/", productHandler.Create)
	r.Get("/", productHandler.List)
	r.Get("/active", productHandler.GetActive)
	r.Get("/by-name/{productName}", productHandler.GetByName)

	r.Get("/{productID}", productHandler.GetById)
	r.Put("/{productID}", productHandler.UpdateById)
	r.Put("/{productID}/active", productHandler.SetActive)
	r.Delete("/{productID}", productHandler.DeleteById)
	r.Get("/{productID}/performance-obligations", bundleHandler.GetPerformObInProds)
	r.Get("/{productID}/bundles", bundleHandler.GetBunsWithProd)
}

func (a *App) loadBundleRoutes(r chi.Router) {
	bundleHandler := &handler.Bundle{DB: a.db}

	r.Post("/", bundleHandler.Create)
	r.Get("/", bundleHandler.List)
	r.Get("/active", bundleHandler.GetActive)
	r.Get("/by-name/{bundleName}", bundleHandler.GetByName)

	r.Get("/{bundleID}", bundleHandler.GetByID)
	r.Put("/{bundleID}", bundleHandler.UpdateById)
	r.Delete("/{bundleID}", bundleHandler.DeleteByID)
	r.Put("/{bundleID}/active", bundleHandler.SetBundleActiveStatus)

	r.Put("/{bundleID}/products/{productID}", bundleHandler.AddProdToBun)
	r.Delete("/{bundleID}/products/{productID}", bundleHandler.DeleteProdFromBun)
	r.Get("/{bundleID}/products", bundleHandler.GetProdsInBun)
	r.Get("/{bundleID}/products/detail", bundleHandler.GetProdsInBunDetail)
	r.Delete("/{bundleID}/products", bundleHandler.ClearProdsFromBun)
	r.Get("/{bundleID}/performance-obligations", bundleHandler.GetPerformObInBuns)
}

func (a *App) loadPerformanceObRoutes(r chi.Router) {
	performanceObHandler := &handler.PerformanceObligation{DB: a.db}
	bundleHandler := &handler.Bundle{DB: a.db}

	r.Get("/", performanceObHandler.List)
	r.Get("/{performanceObID}", performanceObHandler.GetById)
	r.Delete("/{performanceObID}", performanceObHandler.DeleteById)

	r.Route("/{performanceObID}/products", func(r chi.Router) {
		r.Get("/", bundleHandler.GetProdsInPerformOb)
		r.Put("/{productID}", bundleHandler.AddProdToPerformOb)
		r.Delete("/{productID}", bundleHandler.DeleteProdToPerformOb)
		r.Delete("/", bundleHandler.ClearProdsFromPerformOb)
	})

	r.Route("/{performanceObID}/bundles", func(r chi.Router) {
		r.Get("/", bundleHandler.GetBunsInPerformOb)
		r.Put("/{bundleID}", bundleHandler.AddBunToPerformOb)
		r.Delete("/{bundleID}", bundleHandler.DeleteBunToPerformOb)
		r.Delete("/", bundleHandler.ClearBunsFromPerformOb)
	})
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

	performanceObHandler := &handler.PerformanceObligation{DB: a.db}
	r.Get("/performance-obligations", performanceObHandler.ListAll)
	r.Delete("/performance-obligations", performanceObHandler.ResetTable)

	bundleHandler := &handler.Bundle{DB: a.db}
	r.Get("/bundles", bundleHandler.ListAll)
	r.Delete("/bundles", bundleHandler.ResetTableBun)
	r.Delete("/bundle-products", bundleHandler.ResetTableProdBun)
	r.Delete("/performance-obligation-products", bundleHandler.ResetTableProdPerformOb)
	r.Delete("/performance-obligation-bundles", bundleHandler.ResetTableBunPerformOb)
}
