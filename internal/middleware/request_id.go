package middleware

import "net/http"

// RequestID es un placeholder.
// En el router usamos chi/middleware.RequestID.
// Este archivo existe para evitar EOF mientras no lo uses.
func RequestID(next http.Handler) http.Handler {
	return next
}
