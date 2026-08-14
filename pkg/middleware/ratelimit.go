package middleware

import (
	"net"
	"net/http"
	"os"
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

// trustedProxyCIDR is the CIDR of the trusted reverse proxy (e.g. Nginx on aaPanel).
// Set TRUSTED_PROXY env var to your proxy IP/CIDR (e.g. "127.0.0.1/32" or "10.0.0.0/8").
// When not set, X-Forwarded-For is NEVER trusted — RemoteAddr is used exclusively.
// This prevents clients from spoofing their IP to bypass rate limits.
var trustedProxyCIDR = func() *net.IPNet {
	cidr := strings.TrimSpace(os.Getenv("TRUSTED_PROXY"))
	if cidr == "" {
		return nil
	}
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil
	}
	return network
}()

// clientIP returns the real client IP. Only trusts X-Forwarded-For when the
// direct connection comes from the configured TRUSTED_PROXY CIDR.
func clientIP(r *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr without port (shouldn't happen in Go's http.Server, but be safe)
		remoteHost = r.RemoteAddr
	}

	if trustedProxyCIDR != nil {
		remoteIP := net.ParseIP(remoteHost)
		if remoteIP != nil && trustedProxyCIDR.Contains(remoteIP) {
			// This request came through our trusted proxy — trust the forwarded header.
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				// Take only the first (leftmost) IP — that is the real client.
				return strings.TrimSpace(strings.Split(fwd, ",")[0])
			}
		}
	}

	// Direct connection or untrusted origin — RemoteAddr is ground truth.
	return remoteHost
}

