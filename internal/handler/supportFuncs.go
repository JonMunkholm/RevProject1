package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

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
	if _, err := w.Write(dat); err != nil {
		log.Printf("failed to write response: %v", err)
	}
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

func processRequest[Req, Arg, Res interface{}](
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	build func() Req,
	transform func(Req) (Arg, error),
	act func(context.Context, Arg) (Res, error),
	status int,
) (Res, error) {
	req := build()
	if err := decodeJSON(r, &req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid payload", err)
		var zero Res
		return zero, err
	}

	arg, err := transform(req)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "invalid payload", err)
		var zero Res
		return zero, err
	}

	result, err := act(ctx, arg)
	if err != nil {
		status := http.StatusInternalServerError
		msg := "action failed"
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			msg = "resource not found"
		}

		RespondWithError(w, status, msg, err)
		return result, err
	}

	RespondWithJSON(w, status, result)
	return result, nil
}

// ParseUUIDParam extracts and parses a UUID from a chi URL parameter.
// Returns the parsed UUID or an error if the parameter is missing or invalid.
func ParseUUIDParam(r *http.Request, param string) (uuid.UUID, error) {
	raw := chi.URLParam(r, param)
	if raw == "" {
		return uuid.Nil, fmt.Errorf("%s is required", param)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s: %w", param, err)
	}
	return id, nil
}

// RequireUUIDParam extracts a UUID from a chi URL parameter and writes an error
// response if parsing fails. Returns the parsed UUID and true on success, or
// uuid.Nil and false if an error response was written.
func RequireUUIDParam(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := ParseUUIDParam(r, param)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, fmt.Sprintf("invalid %s", param), err)
		return uuid.Nil, false
	}
	return id, true
}
