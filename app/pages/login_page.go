package pages

import (
	"context"
	"io"

	"github.com/JonMunkholm/RevProject1/app/layout"
	"github.com/a-h/templ"
)

func LoginPage() templ.Component {
	return layout.LayoutWithAssets(
		"Sign In • RevProject",
		[]string{"/assets/css/auth.css", "/assets/css/login.css"},
		templ.ComponentFunc(renderLoginContent),
	)
}

func renderLoginContent(ctx context.Context, w io.Writer) error {
	_, err := io.WriteString(w, loginHTML)
	return err
}

const loginHTML = `
<main class="auth-page" role="main">
    <section class="auth-card" id="auth-card">
        <header class="auth-header">
            <h1>Sign in</h1>
            <p>
                New to RevProject?
                <a href="/register">Create an account</a>
            </p>
        </header>

        <div id="login-message" class="auth-feedback" aria-live="polite" role="status"></div>

        <form
            class="auth-form"
            hx-post="/auth/login"
            hx-target="#login-message"
            hx-swap="innerHTML"
            hx-indicator="#login-indicator"
            hx-vals="js:{ timezoneOffset: new Date().getTimezoneOffset() }"
            novalidate
        >
            <div class="form-field">
                <label for="login-email">Email address</label>
                <input
                    id="login-email"
                    type="email"
                    name="email"
                    autocomplete="email"
                    required
                    placeholder="name@company.com"
                />
            </div>

            <div class="form-field">
                <label for="login-password">Password</label>
                <input
                    id="login-password"
                    type="password"
                    name="password"
                    autocomplete="current-password"
                    required
                    minlength="8"
                    placeholder="Enter your password"
                />
            </div>

            <button type="submit" class="primary-button">Sign in</button>

            <div id="login-indicator" class="auth-indicator" aria-live="polite" aria-hidden="true">
                <span class="spinner" aria-hidden="true"></span>
                <span>Checking credentials…</span>
            </div>
        </form>

        <div class="auth-separator" role="presentation">
            <span>Or with</span>
        </div>

        <button type="button" class="sso-button" name="sso" value="true">Continue with Single Sign-On</button>

        <div class="auth-links">
            <a href="/forgot-password">Forgot password?</a>
            <span aria-hidden="true">·</span>
            <a href="/support">Support</a>
            <span aria-hidden="true">·</span>
            <a href="/legal/privacy">Privacy</a>
        </div>
    </section>
</main>
`
