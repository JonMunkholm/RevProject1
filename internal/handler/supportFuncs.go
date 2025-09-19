package handler

import (
	"context"
	"encoding/json"
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
	RespondWithJSON(w, code, struct{Error string `json:"error"`}{Error: msg})
}

func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(payload)
	if err != nil {
        RespondWithError(w, http.StatusInternalServerError, "failed to encode response", err)
        return
    }
	w.WriteHeader(code)
	w.Write(dat)
}

func decodeJSON(r *http.Request, dst any) error {
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()
    return dec.Decode(dst)
}

func pathUUID(r *http.Request, key string) (uuid.UUID, error) {
    raw := chi.URLParam(r, key)
    return uuid.Parse(raw)
}


func createRecord[Req, Arg, Res interface{}](
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
        RespondWithError(w, http.StatusInternalServerError, "action failed", err)
        return result, err
    }

    RespondWithJSON(w, status, result)
    return result, nil
}
