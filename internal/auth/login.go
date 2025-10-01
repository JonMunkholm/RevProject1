package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/sqlc-dev/pqtype"
)

const (
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 60 * 24 * time.Hour
)

const refreshCookiePath = "/auth/refresh"

var (
	errUserInactive    = errors.New("user inactive")
	errCompanyInactive = errors.New("company inactive")
)

type loginPayload struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerPayload struct {
	CompanyName     string `json:"companyName"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

type Login struct {
	DB              *database.Queries
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func (l *Login) SignIn(w http.ResponseWriter, r *http.Request) {
	email, password, err := parseLoginPayload(r)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid login payload", err)
		return
	}

	user, err := l.loadActiveUser(r.Context(), func(ctx context.Context) (database.User, error) {
		return l.DB.GetUserByEmailGlobal(ctx, email)
	})
	if err != nil {
		RespondWithError(w, http.StatusUnauthorized, "incorrect email or password", err)
		return
	}

	if err := CheckPasswordHash(password, user.PasswordHash); err != nil {
		RespondWithError(w, http.StatusUnauthorized, "incorrect email or password", err)
		return
	}

	if err := l.issueSession(w, r, user); err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to issue session", err)
		return
	}

	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/app/dashboard")
		RespondWithJSON(w, http.StatusOK, map[string]string{"message": "login successful"})
		return
	}

	http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
}

func (l *Login) Register(w http.ResponseWriter, r *http.Request) {
	companyName, email, password, confirmPassword, err := parseRegisterPayload(r)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid registration payload", err)
		return
	}

	if password != confirmPassword {
		RespondWithError(w, http.StatusBadRequest, "passwords do not match", errors.New("password confirmation mismatch"))
		return
	}

	if !isValidEmail(email) {
		RespondWithError(w, http.StatusBadRequest, "invalid email format", errors.New("invalid email"))
		return
	}

	if len(password) < 8 {
		RespondWithError(w, http.StatusBadRequest, "password must be at least 8 characters", errors.New("password too short"))
		return
	}

	hashed, err := HashPassword(password)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to hash password", err)
		return
	}

	ctx := r.Context()

	company, err := l.DB.CreateCompany(ctx, companyName)
	if err != nil {
		status, msg, wrapErr := classifyUniqueViolation(err, "company already exists", "failed to create company", err)
		RespondWithError(w, status, msg, wrapErr)
		return
	}

	user, err := l.DB.CreateUser(ctx, database.CreateUserParams{
		CompanyID:    company.ID,
		Email:        email,
		PasswordHash: hashed,
	})
	if err != nil {
		origErr := err
		if delErr := l.DB.DeleteCompany(ctx, company.ID); delErr != nil {
			// best effort cleanup; log for debugging
			err = fmt.Errorf("create user failed: %w (cleanup error: %v)", err, delErr)
		}
		status, msg, wrapErr := classifyUniqueViolation(origErr, "email already registered", "failed to create user", err)
		RespondWithError(w, status, msg, wrapErr)
		return
	}

	if err := l.issueSession(w, r, user); err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to issue session", err)
		return
	}

	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", "/app/dashboard")
		RespondWithJSON(w, http.StatusCreated, map[string]string{"message": "🎉 Registration complete! You're signed in and ready to go."})
		return
	}

	http.Redirect(w, r, "/app/dashboard", http.StatusSeeOther)
}

func (l *Login) issueSession(w http.ResponseWriter, r *http.Request, user database.User) error {
	accessTokenTTL := l.accessTTL()
	refreshTokenTTL := l.refreshTTL()

	jwtPayload := JWTreq{
		UserID:    user.ID,
		CompanyID: user.CompanyID,
		Role:      "",
	}

	accessToken, err := MakeJWT(jwtPayload, l.JWTSecret, accessTokenTTL)
	if err != nil {
		return err
	}

	refreshToken, err := MakeRefreshToken()
	if err != nil {
		return err
	}

	hashedRefresh, err := HashString(refreshToken)
	if err != nil {
		return err
	}

	issuedIP := clientInet(r)
	userAgent := nullString(r.UserAgent())

	err = l.DB.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		UserID:    user.ID,
		TokenHash: hashedRefresh,
		IssuedIp:  issuedIP,
		UserAgent: userAgent,
		ExpiresAt: time.Now().UTC().Add(refreshTokenTTL),
	})
	if err != nil {
		return err
	}

	secureCookie := r.TLS != nil
	accessCookie := &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(accessTokenTTL),
	}
	refreshCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     refreshCookiePath,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(refreshTokenTTL),
	}

	http.SetCookie(w, accessCookie)
	http.SetCookie(w, refreshCookie)

	return nil
}

func (l *Login) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	refreshCookie, err := r.Cookie("refresh_token")
	if err != nil || refreshCookie.Value == "" {
		RespondWithError(w, http.StatusUnauthorized, "refresh token missing", err)
		return
	}

	hashed, err := HashString(refreshCookie.Value)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to process refresh token", err)
		return
	}

	tokenRecord, err := l.DB.GetRefreshTokenByHash(ctx, database.GetRefreshTokenByHashParams{
		TokenHash:      hashed,
		IncludeRevoked: false,
	})
	if err != nil {
		status := http.StatusInternalServerError
		msg := "failed to lookup refresh token"
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusUnauthorized
			msg = "invalid refresh token"
		}
		RespondWithError(w, status, msg, err)
		return
	}
	defer l.revokeRefreshToken(ctx, tokenRecord.ID)

	if tokenRecord.RevokedAt.Valid {
		RespondWithError(w, http.StatusUnauthorized, "refresh token revoked", errors.New("refresh token revoked"))
		return
	}

	now := time.Now().UTC()
	if now.After(tokenRecord.ExpiresAt) {
		RespondWithError(w, http.StatusUnauthorized, "refresh token expired", errors.New("refresh token expired"))
		return
	}

	user, err := l.loadActiveUser(ctx, func(ctx context.Context) (database.User, error) {
		return l.DB.GetUserByIDGlobal(ctx, tokenRecord.UserID)
	})
	if err != nil {
		status := http.StatusUnauthorized
		msg := "failed to load user"
		switch {
		case errors.Is(err, sql.ErrNoRows):
			msg = "user not found"
		case errors.Is(err, errUserInactive):
			status = http.StatusForbidden
			msg = "user inactive"
		case errors.Is(err, errCompanyInactive):
			status = http.StatusForbidden
			msg = "company inactive"
		}
		RespondWithError(w, status, msg, err)
		return
	}

	if err := l.issueSession(w, r, user); err != nil {
		RespondWithError(w, http.StatusInternalServerError, "failed to issue session", err)
		return
	}

	RespondWithJSON(w, http.StatusOK, map[string]string{"message": "session refreshed"})
}

func (l *Login) Logout(w http.ResponseWriter, r *http.Request) {
	secureCookie := r.TLS != nil
	clearCookie := func(name, path string) {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     path,
			HttpOnly: true,
			Secure:   secureCookie,
			SameSite: http.SameSiteStrictMode,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
		})
	}

	if l != nil && l.DB != nil {
		if refreshCookie, err := r.Cookie("refresh_token"); err == nil && refreshCookie.Value != "" {
			hashed, hashErr := HashString(refreshCookie.Value)
			if hashErr != nil {
				log.Printf("failed to hash refresh token during logout: %v", hashErr)
			} else {
				token, lookupErr := l.DB.GetRefreshTokenByHash(r.Context(), database.GetRefreshTokenByHashParams{
					TokenHash:      hashed,
					IncludeRevoked: true,
				})
				if lookupErr != nil {
					if !errors.Is(lookupErr, sql.ErrNoRows) {
						log.Printf("failed to load refresh token for logout: %v", lookupErr)
					}
				} else {
					l.revokeRefreshToken(r.Context(), token.ID)
				}
			}
		}
	}

	clearCookie("access_token", "/")
	clearCookie("refresh_token", refreshCookiePath)

	RespondWithJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (l *Login) loadActiveUser(ctx context.Context, lookup func(context.Context) (database.User, error)) (database.User, error) {
	user, err := lookup(ctx)
	if err != nil {
		return database.User{}, err
	}

	if !user.IsActive {
		return database.User{}, errUserInactive
	}

	company, err := l.DB.GetCompany(ctx, user.CompanyID)
	if err != nil {
		return database.User{}, err
	}

	if !company.IsActive {
		return database.User{}, errCompanyInactive
	}

	return user, nil
}

func (l *Login) revokeRefreshToken(ctx context.Context, id uuid.UUID) {
	if err := l.DB.RevokeRefreshToken(ctx, database.RevokeRefreshTokenParams{ID: id}); err != nil {
		log.Printf("failed to revoke refresh token %s: %v", id, err)
	}
}

func (l *Login) accessTTL() time.Duration {
	if l != nil && l.AccessTokenTTL > 0 {
		return l.AccessTokenTTL
	}
	return defaultAccessTokenTTL
}

func (l *Login) refreshTTL() time.Duration {
	if l != nil && l.RefreshTokenTTL > 0 {
		return l.RefreshTokenTTL
	}
	return defaultRefreshTokenTTL
}

func decodeInto[T any](r *http.Request, dst *T, loadForm func(*T) error) error {
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
			return err
		}
		return nil
	}

	if err := r.ParseForm(); err != nil {
		return err
	}
	return loadForm(dst)
}

func classifyUniqueViolation(err error, conflictMsg, defaultMsg string, wrap error) (int, string, error) {
	if err == nil {
		return http.StatusOK, "", nil
	}
	if isUniqueViolation(err) {
		return http.StatusConflict, conflictMsg, err
	}
	return http.StatusInternalServerError, defaultMsg, wrap
}

func parseLoginPayload(r *http.Request) (string, string, error) {
	payload := loginPayload{}
	if err := decodeInto(r, &payload, func(dst *loginPayload) error {
		dst.Email = strings.TrimSpace(r.FormValue("email"))
		dst.Password = r.FormValue("password")
		if dst.Email == "" || dst.Password == "" {
			return errors.New("missing email or password")
		}
		return nil
	}); err != nil {
		return "", "", err
	}

	payload.Email = strings.TrimSpace(payload.Email)
	if payload.Email == "" || payload.Password == "" {
		return "", "", errors.New("missing email or password")
	}

	return payload.Email, payload.Password, nil
}

func parseRegisterPayload(r *http.Request) (string, string, string, string, error) {
	payload := registerPayload{}
	if err := decodeInto(r, &payload, func(dst *registerPayload) error {
		dst.CompanyName = strings.TrimSpace(r.FormValue("companyName"))
		dst.Email = strings.TrimSpace(r.FormValue("email"))
		dst.Password = r.FormValue("password")
		dst.ConfirmPassword = r.FormValue("confirmPassword")
		if dst.CompanyName == "" || dst.Email == "" || dst.Password == "" || dst.ConfirmPassword == "" {
			return errors.New("missing required registration fields")
		}
		return nil
	}); err != nil {
		return "", "", "", "", err
	}

	payload.CompanyName = strings.TrimSpace(payload.CompanyName)
	payload.Email = strings.TrimSpace(payload.Email)
	if payload.CompanyName == "" || payload.Email == "" || payload.Password == "" || payload.ConfirmPassword == "" {
		return "", "", "", "", errors.New("missing required registration fields")
	}

	return payload.CompanyName, payload.Email, payload.Password, payload.ConfirmPassword, nil
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return false
}

func isValidEmail(email string) bool {
	at := strings.IndexRune(email, '@')
	if at <= 0 || at >= len(email)-1 {
		return false
	}
	return strings.Contains(email[at+1:], ".")
}

func isHTMXRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func clientInet(r *http.Request) pqtype.Inet {
	ip := clientIP(r)
	if ip == nil {
		return pqtype.Inet{}
	}

	maskBits := len(ip) * 8
	ipNet := &net.IPNet{IP: ip, Mask: net.CIDRMask(maskBits, maskBits)}
	return pqtype.Inet{IPNet: *ipNet, Valid: true}
}

func clientIP(r *http.Request) net.IP {
	header := r.Header.Get("X-Forwarded-For")
	if header != "" {
		for _, candidate := range strings.Split(header, ",") {
			ip := net.ParseIP(strings.TrimSpace(candidate))
			if ip != nil {
				return ip
			}
		}
	}

	header = strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if header != "" {
		if ip := net.ParseIP(header); ip != nil {
			return ip
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
