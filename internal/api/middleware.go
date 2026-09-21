package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/k07g/g5/internal/auth"
)

type contextKey string

const contextKeyCognitoSub contextKey = "cognitoSub"

// AuthMiddleware validates the bearer access token on protected routes by
// resolving it against the configured auth.Verifier, and attaches the
// caller's Cognito sub to the request context.
func AuthMiddleware(verifier auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}

			identity, err := verifier.VerifyToken(r.Context(), token)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or expired access token")
				return
			}

			ctx := context.WithValue(r.Context(), contextKeyCognitoSub, identity.Sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}
