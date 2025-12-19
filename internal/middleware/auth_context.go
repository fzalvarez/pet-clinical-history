package middleware

import (
	"context"
	"net/http"
	"strings"

	"pet-clinical-history/internal/ports/auth"
)

type ctxKey string

const claimsKey ctxKey = "claims"

// AuthContext:
// - Si verifier != nil y viene Bearer token => intenta Verify() y setea claims.
// - Si verifier == nil => modo dev: si viene header X-Debug-User-ID => setea claims.
// - Si no hay claims, el request sigue igual; los handlers decidirán si exigen auth.
func AuthContext(verifier auth.AuthVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Dev mode: permitir inyectar user sin verifier
			if verifier == nil {
				if uid := strings.TrimSpace(r.Header.Get("X-Debug-User-ID")); uid != "" {
					claims := auth.Claims{UserID: uid}
					ctx := context.WithValue(r.Context(), claimsKey, claims)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}

				next.ServeHTTP(w, r)
				return
			}

			// Verifier mode
			token := bearerToken(r.Header.Get("Authorization"))
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := verifier.Verify(r.Context(), token)
			if err != nil {
				// No cortamos aquí para no acoplar. El handler decide 401/403.
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetClaims(ctx context.Context) (auth.Claims, bool) {
	v := ctx.Value(claimsKey)
	if v == nil {
		return auth.Claims{}, false
	}
	c, ok := v.(auth.Claims)
	return c, ok
}

func bearerToken(authHeader string) string {
	if strings.TrimSpace(authHeader) == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
