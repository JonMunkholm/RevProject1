package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/JonMunkholm/RevProject1/internal/database"
	"github.com/JonMunkholm/RevProject1/internal/handler"
)

type Login struct {
	DB *database.Queries
	jwtSecret string
}

// Login issues both access + refresh tokens
func (l *Login) SignIn(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		handler.RespondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}

	user, err := l.DB.GetUserByEmail(params.Email)
	if err != nil {
		handler.RespondWithError(w, http.StatusUnauthorized, "Incorrect email or password", err)
		return
	}

	err = CheckPasswordHash(params.Password, user.Password)
	if err != nil {
		handler.RespondWithError(w, http.StatusUnauthorized, "Incorrect email or password", err)
		return
	}

	//make JWT
	req := JWTreq{
		UserID: user.ID,
		CompanyID: user.CompanyID,
		Role: user.Role,
	}

	access, err := MakeJWT(req, l.jwtSecret)
	if err != nil {
		handler.RespondWithError(w, http.StatusInternalServerError, "Couldn't create access JWT", err)
		return
	}

	//make Refresh token
	refresh, err := MakeRefreshToken()
	if err != nil {
		handler.RespondWithError(w, http.StatusInternalServerError, "Couldn't create refresh token", err)
		return
	}
	//something wrong here
	_, err = l.DB.CreateRefreshToken(database.CreateRefreshTokenParams{
		UserID:    user.ID,
		Token:     refresh,
		ExpiresAt: time.Now().UTC().Add(time.Hour * 24 * 60),
	})
	if err != nil {
		handler.RespondWithError(w, http.StatusInternalServerError, "Couldn't save refresh token", err)
		return
	}


	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    access,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // true in production (requires HTTPS)
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		Path:     "/refresh", // only sent on /refresh
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	w.Write([]byte("Login successful!\n"))
}

func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}

	return nil
}

func processAuthReq [Req, Arg, Res interface{}](
	ctx context.Context,
	r *http.Request,
	build func() Req,
	transform func(Req) (Arg, error),
	act func(context.Context, Arg) (Res, error),
	status int,
) (Res, error) {
	req := build()
	if err := decodeJSON(r, &req); err != nil {
		var zero Res
		return zero, fmt.Errorf("invalid payload:", err)
	}

	arg, err := transform(req)
	if err != nil {
		var zero Res
		return zero, fmt.Errorf("invalid payload:", err)
	}

	result, err := act(ctx, arg)
	if err != nil {
		msg := "action failed"
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			msg = "resource not found"
		}

		return result, fmt.Errorf(msg, err)
	}

	RespondWithJSON(w, status, result)
	return result, nil
}
