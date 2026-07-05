package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"flowwithlit/pkg/response"
)

type rateVisitor struct {
	count       int
	windowStart time.Time
}

// RateLimit caps each client IP to maxRequests per window. In-memory only — fine for
// a single backend instance; if this is ever scaled to multiple instances, swap the
// store for something shared (e.g. Redis) so limits are enforced across instances.
func RateLimit(maxRequests int, window time.Duration) func(http.Handler) http.Handler {
	var mu sync.Mutex
	visitors := make(map[string]*rateVisitor)

	go func() {
		for {
			time.Sleep(window)
			mu.Lock()
			for ip, v := range visitors {
				if time.Since(v.windowStart) > window {
					delete(visitors, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)

			mu.Lock()
			v, ok := visitors[ip]
			if !ok || time.Since(v.windowStart) > window {
				v = &rateVisitor{windowStart: time.Now()}
				visitors[ip] = v
			}
			v.count++
			blocked := v.count > maxRequests
			mu.Unlock()

			if blocked {
				response.Error(w, http.StatusTooManyRequests, "Too many requests — please slow down and try again shortly.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return r.RemoteAddr
}
