package pages

import (
	"context"
	"io"

	"github.com/JonMunkholm/RevProject1/app/layout"
	"github.com/a-h/templ"
)

func LandingPage() templ.Component {
	return layout.LayoutWithAssets(
		"RevProject Portal",
		[]string{"/assets/css/landing.css"},
		templ.ComponentFunc(renderLandingContent),
	)
}

func renderLandingContent(ctx context.Context, w io.Writer) error {
	_, err := io.WriteString(w, landingHTML)
	return err
}

const landingHTML = `
<main>
    <section class="hero">
        <span class="hero-kicker">Revenue Operations Platform</span>
        <h1>Bring companies, customers, products, and bundles together.</h1>
        <p>
            RevProject orchestrates your revenue workflow with a single contract command center.
            Track fulfillment, manage performance obligations, and align finance and GTM teams without duct tape spreadsheets.
        </p>
        <ul>
            <li>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M20 6L9 17l-5-5" />
                </svg>
                Contract-centric workspace with real-time status changes.
            </li>
            <li>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M3 6h18M3 12h12M3 18h6" />
                </svg>
                Coordinated updates flow through HTMX-powered UI fragments.
            </li>
            <li>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M12 20l9-5-9-5-9 5 9 5z" />
                    <path d="M12 12l9-5-9-5-9 5 9 5z" />
                </svg>
                Secure JWT-backed sessions across admin and operating teams.
            </li>
        </ul>
    </section>

    <section class="card highlight-card">
        <header>
            <h2>Power your revenue teams</h2>
            <p class="card-subtitle">
                Spin up RevProject, connect your contracts, and invite operators when you are ready.
            </p>
        </header>

        <dl class="capability-metrics">
            <div>
                <dt>Contracts managed</dt>
                <dd>Instant visibility into performance obligations per customer.</dd>
            </div>
            <div>
                <dt>Roles aligned</dt>
                <dd>Owners, operators, and admins stay in sync with fine-grained permissions.</dd>
            </div>
            <div>
                <dt>Admin tooling</dt>
                <dd>Reset data, review audit logs, and seed demo-tenants without touching SQL.</dd>
            </div>
        </dl>
    </section>

    <section class="card login-cta">
        <h2>Ready to put RevProject to work?</h2>
        <p>
            Secure sign-in routes you straight to the operational console. Bring your company
            ID and team credentials to keep your revenue process connected.
        </p>
        <div class="cta-actions">
            <a class="primary-button" href="/login">Go to login</a>
            <p>
                New here? <a href="/register" class="form-link">Create your company workspace</a> and be up and running in minutes.
            </p>
        </div>
    </section>

    <section class="feature-grid" aria-labelledby="platform-feature-heading">
        <header>
            <h2 id="platform-feature-heading">Built for RevOps orchestration</h2>
            <p>Every capability maps directly to the APIs already shipping in this codebase.</p>
        </header>
        <ul>
            <li>
                <h3>Model companies and hierarchies</h3>
                <p>Register new companies, set active status, and invite operators with role-aware access controls.</p>
                <button class="ghost-button" hx-get="/docs/capabilities" data-focus="Model companies and hierarchies" hx-target="#capability-details" hx-trigger="click">
                    View API surface
                </button>
            </li>
            <li>
                <h3>Map customers and contracts</h3>
                <p>Connect customers to contracts, attach supporting performance obligations, and see fulfillment status in one view.</p>
                <button class="ghost-button" hx-get="/docs/capabilities" data-focus="Map customers and contracts" hx-target="#capability-details" hx-trigger="click">
                    View API surface
                </button>
            </li>
            <li>
                <h3>Bundle products with intent</h3>
                <p>Compose products into bundles, reuse obligations, and reuse pricing packages across revenue workflows.</p>
                <button class="ghost-button" hx-get="/docs/capabilities" data-focus="Bundle products with intent" hx-target="#capability-details" hx-trigger="click">
                    View API surface
                </button>
            </li>
            <li>
                <h3>Operational tooling ready</h3>
                <p>Reset seed data, audit recent imports, and manage administrative tasks without leaving the dashboard.</p>
                <button class="ghost-button" hx-get="/docs/capabilities" data-focus="Operational tooling ready" hx-target="#capability-details" hx-trigger="click">
                    View API surface
                </button>
            </li>
        </ul>
    </section>

	<section id="capability-details" class="capability-details" aria-live="polite"></section>
</main>
`
