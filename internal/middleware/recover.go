package middleware

import "net/http"

// Recover es un placeholder.
// Ahora usamos chi/middleware.Recoverer en el router,
// así que este archivo solo existe para no romper compilación.
func Recover(next http.Handler) http.Handler {
	return next
}
