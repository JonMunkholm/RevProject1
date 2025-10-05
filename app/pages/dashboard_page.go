package pages

import (
	"context"
	"io"

	"github.com/JonMunkholm/RevProject1/app/layout"
	"github.com/a-h/templ"
)

type dashboardPageConfig struct {
	Title                 string
	Active                string
	Heading               string
	Description           string
	StatusLoadingMessage  string
	RefreshLabel          string
	RefreshPath           string
	RecentTitle           string
	RecentDescription     string
	PlaceholderRowMessage string
}

type navLink struct {
	Route string
	Label string
	Icon  string
}

var dashboardNavLinks = []navLink{
	{Route: "dashboard", Label: "Dashboard", Icon: "DB"},
	{Route: "review", Label: "Review", Icon: "RV"},
	{Route: "customers", Label: "Customers", Icon: "CU"},
	{Route: "products", Label: "Products", Icon: "PR"},
}

type metricCardDefinition struct {
	ID          string
	Label       string
	DetailLabel string
	Note        string
}

var dashboardMetricCards = []metricCardDefinition{
	{ID: "users", Label: "Users", DetailLabel: "Active", Note: "People with access to this workspace."},
	{ID: "customers", Label: "Customers", DetailLabel: "Active", Note: "Customers currently modeled for revenue tracking."},
	{ID: "contracts", Label: "Contracts", DetailLabel: "Finalized", Note: "Total agreements associated with this company."},
	{ID: "products", Label: "Products", DetailLabel: "Active", Note: "Sellable catalog entries supporting current deals."},
	{ID: "bundles", Label: "Bundles", DetailLabel: "Active", Note: "Configured bundles available to assemble offers."},
	{ID: "performance-obligations", Label: "Performance Obligations", Note: "Tracked fulfillment components connected to contracts."},
}

func DashboardPage(active string) templ.Component {
	return dashboardLikePage(dashboardPageConfig{
		Title:                 "Workspace Dashboard",
		Active:                active,
		Heading:               "Workspace Dashboard",
		Description:           "Monitor contract activity and product coverage across your revenue stack.",
		StatusLoadingMessage:  "Loading metrics…",
		RefreshLabel:          "Refresh",
		RefreshPath:           "/app/dashboard",
		RecentTitle:           "Recent Contracts",
		RecentDescription:     "Latest contract updates within your company.",
		PlaceholderRowMessage: "Waiting for contract activity…",
	})
}

func dashboardLikePage(cfg dashboardPageConfig) templ.Component {
	return layout.LayoutWithAssets(
		cfg.Title,
		[]string{"/assets/css/dashboard.css"},
		dashboardShell(cfg),
	)
}

func dashboardShell(cfg dashboardPageConfig) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := write(w, `<div class="dashboard-frame">`); err != nil {
			return err
		}
		if err := DashboardSidebar(cfg.Active).Render(ctx, w); err != nil {
			return err
		}
		if err := dashboardMain(cfg).Render(ctx, w); err != nil {
			return err
		}
		return write(w, `</div>`)
	})
}

func DashboardSidebar(active string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := write(w, `<aside class="side-nav" data-dashboard-nav>`); err != nil {
			return err
		}
		if err := write(w, `<div class="side-nav__brand"><span class="side-nav__logo" aria-hidden="true">RP</span><div class="side-nav__titles"><span class="side-nav__name">RevProject</span><span class="side-nav__tag">Operations</span></div></div>`); err != nil {
			return err
		}
		if err := dashboardNav(active).Render(ctx, w); err != nil {
			return err
		}
		if err := write(w, `<div class="side-nav__footer">`); err != nil {
			return err
		}
		if err := write(w, `<a href="/app/settings" class="side-nav__link" data-route="settings"`); err != nil {
			return err
		}
		if active == "settings" {
			if err := write(w, ` aria-current="page"`); err != nil {
				return err
			}
		}
		if err := write(w, `><span class="side-nav__icon" aria-hidden="true">ST</span><span class="side-nav__text">Settings</span></a>`); err != nil {
			return err
		}
		if err := write(w, `<form method="post" action="/auth/logout" class="side-nav__logout" id="logout-form">`); err != nil {
			return err
		}
		if err := write(w, `<button type="submit" class="side-nav__link side-nav__link--button" id="logout-button">`); err != nil {
			return err
		}
		if err := write(w, `<span class="side-nav__icon" aria-hidden="true">LO</span><span class="side-nav__text">Log out</span></button></form>`); err != nil {
			return err
		}
		return write(w, `</div></aside>`)
	})
}

func dashboardNav(active string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := write(w, `<nav class="side-nav__menu" aria-label="Primary">`); err != nil {
			return err
		}
		for _, link := range dashboardNavLinks {
			route := templ.EscapeString(link.Route)
			label := templ.EscapeString(link.Label)
			icon := templ.EscapeString(link.Icon)

			if err := write(w, `<a href="/app/`+route+`" class="side-nav__link" data-route="`+route+`"`); err != nil {
				return err
			}
			if active == link.Route {
				if err := write(w, ` aria-current="page"`); err != nil {
					return err
				}
			}
			if err := write(w, `><span class="side-nav__icon" aria-hidden="true">`+icon+`</span><span class="side-nav__text">`+label+`</span></a>`); err != nil {
				return err
			}
		}
		return write(w, `</nav>`)
	})
}

func dashboardMain(cfg dashboardPageConfig) templ.Component {
	heading := cfg.Heading
	if heading == "" {
		heading = "Workspace Dashboard"
	}
	description := cfg.Description
	if description == "" {
		description = "Monitor contract activity and product coverage across your revenue stack."
	}
	status := cfg.StatusLoadingMessage
	if status == "" {
		status = "Loading metrics…"
	}
	refreshLabel := cfg.RefreshLabel
	if refreshLabel == "" {
		refreshLabel = "Refresh"
	}

	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := write(w, `<div class="dashboard-shell">`); err != nil {
			return err
		}
		if err := dashboardHeader(heading, description, status, refreshLabel, cfg.RefreshPath).Render(ctx, w); err != nil {
			return err
		}
		if err := dashboardSummary(cfg).Render(ctx, w); err != nil {
			return err
		}
		return write(w, `</div>`)
	})
}

func dashboardHeader(heading, description, status, refreshLabel, refreshPath string) templ.Component {
	escapedHeading := templ.EscapeString(heading)
	escapedDescription := templ.EscapeString(description)
	escapedStatus := templ.EscapeString(status)
	escapedLabel := templ.EscapeString(refreshLabel)

	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := write(w, `<header class="dashboard-header">`); err != nil {
			return err
		}
		if err := write(w, `<div class="dashboard-heading"><h1>`+escapedHeading+`</h1><p>`+escapedDescription+`</p></div>`); err != nil {
			return err
		}
		if err := write(w, `<div class="dashboard-actions"><span class="dashboard-status" id="dashboard-status" data-dashboard-status>`+escapedStatus+`</span>`); err != nil {
			return err
		}
		if refreshPath != "" {
			escapedPath := templ.EscapeString(refreshPath)
			if err := write(w, `<button id="refresh-dashboard" type="button" class="ghost-button" hx-get="`+escapedPath+`" hx-target="#dashboard-summary" hx-select="#dashboard-summary" hx-swap="outerHTML">`+escapedLabel+`</button>`); err != nil {
				return err
			}
		}
		if err := write(w, `</div></header>`); err != nil {
			return err
		}
		return nil
	})
}

func dashboardSummary(cfg dashboardPageConfig) templ.Component {
	recentTitle := cfg.RecentTitle
	if recentTitle == "" {
		recentTitle = "Recent Contracts"
	}
	recentDescription := cfg.RecentDescription
	if recentDescription == "" {
		recentDescription = "Latest contract updates within your company."
	}
	placeholder := cfg.PlaceholderRowMessage
	if placeholder == "" {
		placeholder = "Waiting for contract activity…"
	}

	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := write(w, `<section id="dashboard-summary"`); err != nil {
			return err
		}
		if cfg.RefreshPath != "" {
			escapedPath := templ.EscapeString(cfg.RefreshPath)
			if err := write(w, ` hx-get="`+escapedPath+`" hx-trigger="every 120s" hx-select="#dashboard-summary" hx-target="#dashboard-summary" hx-swap="outerHTML"`); err != nil {
				return err
			}
		}
		if err := write(w, `>`); err != nil {
			return err
		}
		if err := metricsSection().Render(ctx, w); err != nil {
			return err
		}
		if err := recentContractsSection(recentTitle, recentDescription, placeholder).Render(ctx, w); err != nil {
			return err
		}
		return write(w, `</section>`)
	})
}

func metricsSection() templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := write(w, `<section class="metrics-grid" aria-label="Workspace metrics">`); err != nil {
			return err
		}
		for _, card := range dashboardMetricCards {
			if err := metricCardComponent(card).Render(ctx, w); err != nil {
				return err
			}
		}
		return write(w, `</section>`)
	})
}

func metricCardComponent(def metricCardDefinition) templ.Component {
	id := templ.EscapeString(def.ID)
	label := templ.EscapeString(def.Label)
	detail := templ.EscapeString(def.DetailLabel)
	note := templ.EscapeString(def.Note)

	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := write(w, `<article class="metric-card" data-metric="`+id+`"><header>`); err != nil {
			return err
		}
		if err := write(w, `<span class="metric-label">`+label+`</span>`); err != nil {
			return err
		}
		if def.DetailLabel != "" {
			if err := write(w, `<span class="metric-subvalue">`+detail+`: <span class="metric-detail-value">–</span></span>`); err != nil {
				return err
			}
		}
		if err := write(w, `</header><div class="metric-value">–</div><p class="metric-note">`+note+`</p></article>`); err != nil {
			return err
		}
		return nil
	})
}

func recentContractsSection(title, description, placeholder string) templ.Component {
	escapedTitle := templ.EscapeString(title)
	escapedDescription := templ.EscapeString(description)
	escapedPlaceholder := templ.EscapeString(placeholder)

	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if err := write(w, `<section class="panel">`); err != nil {
			return err
		}
		if err := write(w, `<header class="panel-header"><div><h2>`+escapedTitle+`</h2><p>`+escapedDescription+`</p></div></header>`); err != nil {
			return err
		}
		if err := write(w, `<div class="panel-body"><table id="recent-contracts" aria-live="polite">`); err != nil {
			return err
		}
		if err := write(w, `<thead><tr><th scope="col">Customer</th><th scope="col">Start</th><th scope="col">End</th><th scope="col">Status</th><th scope="col">Updated</th></tr></thead>`); err != nil {
			return err
		}
		if err := write(w, `<tbody><tr class="placeholder-row"><td colspan="5">`+escapedPlaceholder+`</td></tr></tbody></table></div></section>`); err != nil {
			return err
		}
		return nil
	})
}

func write(w io.Writer, s string) error {
	_, err := io.WriteString(w, s)
	return err
}
