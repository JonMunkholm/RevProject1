package application

import (
	"errors"
	"net/http"
	"strings"

	"github.com/JonMunkholm/RevProject1/app/pages"
	"github.com/JonMunkholm/RevProject1/internal/auth"
)

func (a *App) chatPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := auth.SessionFromContext(r.Context())
		if !ok {
			auth.RespondWithError(w, http.StatusUnauthorized, "authentication required", errors.New("session missing"))
			return
		}

		if a.aiHandler == nil {
			a.aiHandler = a.newAIHandler()
		}

		providerParam := strings.TrimSpace(r.URL.Query().Get("provider"))
		conversationParam := strings.TrimSpace(r.URL.Query().Get("conversation"))

		props, err := a.aiHandler.BuildChatProps(r.Context(), session, providerParam, conversationParam, true)
		if err != nil {
			auth.RespondWithError(w, http.StatusInternalServerError, "failed to load chat", err)
			return
		}

		a.render(w, r, pages.ChatPage(props))
	}
}
