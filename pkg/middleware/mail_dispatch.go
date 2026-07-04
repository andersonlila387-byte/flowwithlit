package middleware

import (
	"crypto/subtle"
	"net/http"

	"flowwithlit/pkg/email"
	"flowwithlit/pkg/response"
)

// RequireMailDispatchSecret protects internal mail test routes (same secret as mail/dispatch.php).
func RequireMailDispatchSecret(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Mail-Secret")
		want := email.MailDispatchSecret()
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
			response.Error(w, http.StatusUnauthorized, "Invalid mail dispatch secret")
			return
		}
		next.ServeHTTP(w, r)
	})
}