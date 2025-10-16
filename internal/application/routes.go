package application

import (
	"net/http"

	"github.com/JonMunkholm/RevProject1/app/pages"
	"github.com/JonMunkholm/RevProject1/internal/auth"
	"github.com/JonMunkholm/RevProject1/internal/database"
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
			a.render(w, r, pages.LandingPage())
		})

		r.Route("/login", a.loadLogin)
		r.Route("/register", a.loadRegister)
		r.Route("/auth", a.loadAuthRoutes)
	})

	// Authenticated application + API surface
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(a.jwtSecret))

		r.Route("/app", func(r chi.Router) {
			r.Get("/", a.dashboardPage("dashboard"))
			r.Get("/dashboard", a.dashboardPage("dashboard"))
			r.Get("/review", a.dashboardPage("review"))
			r.Get("/customers", a.dashboardPage("customers"))
			r.Get("/products", a.dashboardPage("products"))
			r.Route("/settings", a.loadSettingsRoutes)
			r.Route("/chat", a.loadChatRoutes)
		})

		r.Route("/api", func(r chi.Router) {
			r.Route("/dashboard", a.loadDashboardRoutes)
			r.Route("/review", a.loadReviewRoutes)
			r.Route("/companies", a.loadCompanyRoutes)
			r.Route("/admin", a.loadAdminRoutes)
			r.Route("/ai", func(r chi.Router) {
				r.Use(auth.RequireCompanyRole(auth.RoleViewer))
				a.loadAIRoutes(r)
			})
		})
	})

	a.router = r
}

func serveAppAssets(r chi.Router) {
	r.Handle(
		"/assets/*",
		http.StripPrefix(
			"/assets/",
			http.FileServer(http.Dir("app/assets")),
		),
	)
	r.Handle(
		"/fonts/*",
		http.StripPrefix(
			"/fonts/",
			http.FileServer(http.Dir("app/assets/fonts")),
		),
	)
}

func (a *App) render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := component.Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func (a *App) loadLogin(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		a.render(w, r, pages.LoginPage())
	})
}

func (a *App) loadRegister(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		a.render(w, r, pages.RegisterPage())
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
	r.Post("/logout", loginHandler.Logout)
}

func (a *App) dashboardPage(active string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var component templ.Component
		switch active {
		case "review":
			component = pages.ReviewPage(active)
		default:
			component = pages.DashboardPage(active)
		}
		a.render(w, r, component)
	}
}

func (a *App) loadDashboardRoutes(r chi.Router) {
	bindDashboardSummary(a.db, r)
}

func (a *App) loadReviewRoutes(r chi.Router) {
	bindDashboardSummary(a.db, r)
}

func bindDashboardSummary(db *database.Queries, r chi.Router) {
	h := &handler.Dashboard{DB: db}
	r.Get("/summary", h.Summary)
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

func (a *App) loadAIRoutes(r chi.Router) {
	aiHandler := a.newAIHandler()
	a.aiHandler = aiHandler

	r.Post("/conversations", aiHandler.CreateConversation)
	r.Get("/conversations", aiHandler.ListConversations)
	r.Get("/conversations/{sessionID}/messages", aiHandler.ListConversationMessages)
	r.Post("/conversations/{sessionID}/messages", aiHandler.AppendConversationMessage)

	r.Post("/documents/jobs", aiHandler.CreateDocumentJob)
	r.Get("/documents/jobs", aiHandler.ListDocumentJobs)
	r.Get("/documents/jobs/{jobID}", aiHandler.GetDocumentJob)
	r.Get("/providers/catalog", aiHandler.ListProviderCatalog)
	r.Get("/providers", aiHandler.ListProviderCredentials)
	r.Get("/providers/{providerID}/credentials", aiHandler.ListProviderCredentials)
	r.Post("/providers", aiHandler.UpsertProviderCredential)
	r.Post("/providers/test", aiHandler.TestProviderCredential)
	r.Get("/providers/{providerID}/status", aiHandler.ProviderStatus)
	r.Get("/providers/{providerID}/events", aiHandler.ListProviderCredentialEvents)
	r.Post("/providers/{providerID}/credential", aiHandler.UpsertProviderCredential)
	r.Post("/providers/{providerID}/credential/test", aiHandler.TestProviderCredential)
	r.Delete("/credentials/{credentialID}", aiHandler.DeleteProviderCredential)
}

func (a *App) loadChatRoutes(r chi.Router) {
	if a.aiHandler == nil {
		a.aiHandler = a.newAIHandler()
	}

	r.Use(auth.RequireCompanyRole(auth.RoleViewer))

	r.Get("/", a.chatPage())
	r.Get("/conversations", a.aiHandler.ChatListSessions)
	r.Get("/conversations/{sessionID}", a.aiHandler.ChatLoadSession)
	r.Post("/conversations", a.aiHandler.ChatCreateSession)
	r.Post("/conversations/{sessionID}/messages", a.aiHandler.ChatAppendMessage)
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
