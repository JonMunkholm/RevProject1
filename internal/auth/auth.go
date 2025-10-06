package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RoleUnknown Role = ""
	RoleViewer  Role = "viewer"
	RoleMember  Role = "member"
	RoleAdmin   Role = "admin"
)

var roleHierarchy = map[Role]int{
	RoleViewer: 1,
	RoleMember: 2,
	RoleAdmin:  3,
}

func ParseRole(value string) Role {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(RoleAdmin):
		return RoleAdmin
	case string(RoleMember):
		return RoleMember
	case string(RoleViewer):
		return RoleViewer
	default:
		return RoleViewer
	}
}

func (r Role) String() string { return string(r) }

func (r Role) Meets(min Role) bool {
	rank, ok := roleHierarchy[r]
	if !ok {
		return false
	}
	required, ok := roleHierarchy[min]
	if !ok {
		return true
	}
	return rank >= required
}

type JWTreq struct {
	UserID      uuid.UUID
	CompanyID   uuid.UUID
	CurrentRole Role
	Roles       map[uuid.UUID]Role
}

type CustomClaims struct {
	jwt.RegisteredClaims
	CompanyID   string            `json:"companyID"`
	CurrentRole string            `json:"currentRole,omitempty"`
	Roles       map[string]string `json:"roles,omitempty"`
}

type TokenType string

const (
	TokenTypeAccess TokenType = "revProject-1-User"
)

var ErrNoAuthHeaderIncluded = errors.New("no auth header included in request")

func HashPassword(password string) (string, error) {
	dat, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(dat), nil
}

func CheckPasswordHash(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func MakeJWT(req JWTreq, tokenSecret string, expiresIn time.Duration) (string, error) {
	signingKey := []byte(tokenSecret)

	roleClaims := make(map[string]string, len(req.Roles))
	for companyID, role := range req.Roles {
		if role == RoleUnknown {
			continue
		}
		roleClaims[companyID.String()] = role.String()
	}

	claims := CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    string(TokenTypeAccess),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expiresIn)),
			Subject:   req.UserID.String(),
		},
		CompanyID:   req.CompanyID.String(),
		CurrentRole: req.CurrentRole.String(),
		Roles:       roleClaims,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

func ValidateJWT(tokenString, tokenSecret string) (CustomClaims, error) {
	claims := CustomClaims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (interface{}, error) { return []byte(tokenSecret), nil },
	)
	if err != nil {
		return CustomClaims{}, err
	}

	if !token.Valid {
		return CustomClaims{}, errors.New("invalid token")
	}

	issuer, err := claims.GetIssuer()
	if err != nil {
		return CustomClaims{}, err
	}
	if issuer != string(TokenTypeAccess) {
		return CustomClaims{}, errors.New("invalid issuer")
	}

	if _, err := uuid.Parse(claims.Subject); err != nil {
		return CustomClaims{}, fmt.Errorf("invalid subject id: %w", err)
	}

	return claims, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", ErrNoAuthHeaderIncluded
	}

	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("malformed authorization header")
	}

	return parts[1], nil
}

func MakeRefreshToken() (string, error) {
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return "", err
	}
	return hex.EncodeToString(token), nil
}

func HashString(value string) ([]byte, error) {
	sum := sha256.Sum256([]byte(value))
	return sum[:], nil
}

func GetAPIKey(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return "", ErrNoAuthHeaderIncluded
	}

	parts := strings.Fields(authHeader)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "ApiKey") {
		return "", errors.New("malformed authorization header")
	}

	return parts[1], nil
}

func RespondWithError(w http.ResponseWriter, code int, msg string, err error) {
	if err != nil {
		log.Println(err)
	}
	if code > 499 {
		log.Printf("Responding with 5XX error: %s", msg)
	}
	RespondWithJSON(w, code, struct {
		Error string `json:"error"`
	}{Error: msg})
}

func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("failed to encode response: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}
