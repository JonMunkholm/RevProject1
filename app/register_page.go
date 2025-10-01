package app

import (
	"context"
	"io"

	"github.com/a-h/templ"
)

func RegisterPage() templ.Component {
	return LayoutWithAssets("Create Your RevProject Workspace", nil, []string{"/register.js"}, templ.ComponentFunc(renderRegisterContent))
}

func renderRegisterContent(ctx context.Context, w io.Writer) error {
	_, err := io.WriteString(w, registerHTML)
	return err
}

const registerHTML = `
<main class="login-layout">
    <section class="card" id="auth-card">
        <header>
            <h1>Register your company</h1>
            <p class="card-subtitle">
                Create a company workspace, invite yourself in as the first operator, and you'll be signed in automatically.
            </p>
        </header>

        <form
            id="register-form"
            method="post"
            action="/auth/register"
            hx-post="/auth/register"
            hx-target="#register-message"
            hx-swap="innerHTML"
            hx-indicator="#register-indicator"
        >
            <label>
                Company name
                <input
                    type="text"
                    name="companyName"
                    class="input-field"
                    autocomplete="organization"
                    required
                    placeholder="Acme Corporation"
                />
            </label>

            <label>
                Email address
                <input
                    type="email"
                    name="email"
                    class="input-field"
                    autocomplete="email"
                    required
                    placeholder="founder@acme.com"
                />
            </label>

            <label>
                Password
                <input
                    type="password"
                    name="password"
                    class="input-field"
                    autocomplete="new-password"
                    minlength="8"
                    required
                    placeholder="Create a secure password"
                />
            </label>

            <label>
                Confirm password
                <input
                    type="password"
                    name="confirmPassword"
                    class="input-field"
                    autocomplete="new-password"
                    minlength="8"
                    required
                    placeholder="Re-enter your password"
                />
            </label>

            <button type="submit">Create workspace</button>

            <div
                id="register-indicator"
                class="htmx-indicator"
                aria-live="polite"
                aria-hidden="true"
            >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="9" stroke-opacity="0.2" />
                    <path d="M21 12a9 9 0 0 1-9 9" />
                </svg>
                Setting up your workspace…
            </div>

            <p id="register-message" class="message" aria-live="polite"></p>
        </form>

        <div class="alt-actions">
            <span>Already have access?</span>
            <a href="/login">Return to login</a>
        </div>
    </section>
</main>
`
