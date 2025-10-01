package app

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/a-h/templ"
)

type dashboardPageConfig struct {
	Title                 string
	Active                string
	Endpoint              string
	Heading               string
	Description           string
	StatusLoadingMessage  string
	RefreshLabel          string
	RecentTitle           string
	RecentDescription     string
	PlaceholderRowMessage string
}

func DashboardPage(active string) templ.Component {
	return dashboardLikePage(dashboardPageConfig{
		Title:                 "Workspace Dashboard",
		Active:                active,
		Endpoint:              "/api/dashboard/summary",
		Heading:               "Workspace Dashboard",
		Description:           "Monitor contract activity and product coverage across your revenue stack.",
		StatusLoadingMessage:  "Loading metrics…",
		RefreshLabel:          "Refresh",
		RecentTitle:           "Recent Contracts",
		RecentDescription:     "Latest contract updates within your company.",
		PlaceholderRowMessage: "Waiting for contract activity…",
	})
}

func ReviewPage(active string) templ.Component {
	return dashboardLikePage(dashboardPageConfig{
		Title:                 "Review Workspace",
		Active:                active,
		Endpoint:              "/api/review/summary",
		Heading:               "Review Dashboard",
		Description:           "Validate revenue records and contract outcomes before finalizing.",
		StatusLoadingMessage:  "Loading metrics…",
		RefreshLabel:          "Refresh",
		RecentTitle:           "Recent Contracts",
		RecentDescription:     "Latest contract updates within your company.",
		PlaceholderRowMessage: "Waiting for contract activity…",
	})
}

func dashboardLikePage(cfg dashboardPageConfig) templ.Component {
	return LayoutWithAssets(
		cfg.Title,
		[]string{"/dashboard.css"},
		[]string{"/dashboard.js"},
		templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			_, err := io.WriteString(w, dashboardShell(cfg))
			return err
		}),
	)
}

func dashboardShell(cfg dashboardPageConfig) string {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "/api/dashboard/summary"
	}
	nav := buildNav(cfg.Active)
	return "<div class=\"dashboard-frame\" data-dashboard-endpoint=\"" + templ.EscapeString(endpoint) + "\">" +
		"<aside class=\"side-nav\" data-dashboard-nav>" +
		"<div class=\"side-nav__brand\"><span class=\"side-nav__logo\" aria-hidden=\"true\">RP</span><div class=\"side-nav__titles\"><span class=\"side-nav__name\">RevProject</span><span class=\"side-nav__tag\">Operations</span></div></div>" +
		nav +
		"<div class=\"side-nav__footer\"><a href=\"/app/settings\" class=\"side-nav__link\" data-route=\"settings\"" + activeAttr(cfg.Active == "settings") + ">" +
		"<span class=\"side-nav__icon\" aria-hidden=\"true\">ST</span><span class=\"side-nav__text\">Settings</span></a>" +
		"<form method=\"post\" action=\"/auth/logout\" class=\"side-nav__logout\" id=\"logout-form\"><button type=\"submit\" class=\"side-nav__link side-nav__link--button\" id=\"logout-button\"><span class=\"side-nav__icon\" aria-hidden=\"true\">LO</span><span class=\"side-nav__text\">Log out</span></button></form></div></aside>" +
		"<div class=\"dashboard-shell\">" + dashboardMain(cfg) + "</div></div>"
}

func buildNav(active string) string {
	links := []struct {
		Route string
		Label string
		Icon  string
	}{
		{"dashboard", "Dashboard", "DB"},
		{"review", "Review", "RV"},
		{"customers", "Customers", "CU"},
		{"products", "Products", "PR"},
	}

	var b strings.Builder
	b.WriteString("<nav class=\"side-nav__menu\" aria-label=\"Primary\">")
	for _, link := range links {
		b.WriteString("<a href=\"/app/" + link.Route + "\" class=\"side-nav__link\" data-route=\"" + link.Route + "\"" + activeAttr(active == link.Route) + ">")
		b.WriteString("<span class=\"side-nav__icon\" aria-hidden=\"true\">" + link.Icon + "</span>")
		b.WriteString("<span class=\"side-nav__text\">" + link.Label + "</span></a>")
	}
	b.WriteString("</nav>")
	return b.String()
}

func activeAttr(active bool) string {
	if active {
		return " aria-current=\"page\""
	}
	return ""
}

func dashboardMain(cfg dashboardPageConfig) string {
	heading := cfg.Heading
	if heading == "" {
		heading = "Workspace Dashboard"
	}
	description := cfg.Description
	if description == "" {
		description = "Monitor contract activity and product coverage across your revenue stack."
	}
	statusMessage := cfg.StatusLoadingMessage
	if statusMessage == "" {
		statusMessage = "Loading metrics…"
	}
	refreshLabel := cfg.RefreshLabel
	if refreshLabel == "" {
		refreshLabel = "Refresh"
	}
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

	return fmt.Sprintf(`
<header class="dashboard-header">
    <div class="dashboard-heading">
        <h1>%s</h1>
        <p>%s</p>
    </div>
    <div class="dashboard-actions">
        <span class="dashboard-status" data-dashboard-status>%s</span>
        <button id="refresh-dashboard" type="button" class="ghost-button">%s</button>
    </div>
</header>
<section class="metrics-grid">
    <article class="metric-card" data-metric="users">
        <header>
            <span class="metric-label">Users</span>
            <span class="metric-subvalue">Active: <span class="metric-detail-value">–</span></span>
        </header>
        <div class="metric-value">–</div>
        <p class="metric-note">People with access to this workspace.</p>
    </article>
    <article class="metric-card" data-metric="customers">
        <header>
            <span class="metric-label">Customers</span>
            <span class="metric-subvalue">Active: <span class="metric-detail-value">–</span></span>
        </header>
        <div class="metric-value">–</div>
        <p class="metric-note">Customers currently modeled for revenue tracking.</p>
    </article>
    <article class="metric-card" data-metric="contracts">
        <header>
            <span class="metric-label">Contracts</span>
            <span class="metric-subvalue">Finalized: <span class="metric-detail-value">–</span></span>
        </header>
        <div class="metric-value">–</div>
        <p class="metric-note">Total agreements associated with this company.</p>
    </article>
    <article class="metric-card" data-metric="products">
        <header>
            <span class="metric-label">Products</span>
            <span class="metric-subvalue">Active: <span class="metric-detail-value">–</span></span>
        </header>
        <div class="metric-value">–</div>
        <p class="metric-note">Sellable catalog entries supporting current deals.</p>
    </article>
    <article class="metric-card" data-metric="bundles">
        <header>
            <span class="metric-label">Bundles</span>
            <span class="metric-subvalue">Active: <span class="metric-detail-value">–</span></span>
        </header>
        <div class="metric-value">–</div>
        <p class="metric-note">Configured bundles available to assemble offers.</p>
    </article>
    <article class="metric-card" data-metric="performance-obligations">
        <header>
            <span class="metric-label">Performance Obligations</span>
        </header>
        <div class="metric-value">–</div>
        <p class="metric-note">Tracked fulfillment components connected to contracts.</p>
    </article>
</section>
<section class="panel">
    <header class="panel-header">
        <div>
            <h2>%s</h2>
            <p>%s</p>
        </div>
    </header>
    <div class="panel-body">
        <table id="recent-contracts">
            <thead>
                <tr>
                    <th scope="col">Customer</th>
                    <th scope="col">Start</th>
                    <th scope="col">End</th>
                    <th scope="col">Status</th>
                    <th scope="col">Updated</th>
                </tr>
            </thead>
            <tbody>
                <tr class="placeholder-row">
                    <td colspan="5">%s</td>
                </tr>
            </tbody>
        </table>
    </div>
</section>
`,
		templ.EscapeString(heading),
		templ.EscapeString(description),
		templ.EscapeString(statusMessage),
		templ.EscapeString(refreshLabel),
		templ.EscapeString(recentTitle),
		templ.EscapeString(recentDescription),
		templ.EscapeString(placeholder),
	)
}
