package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ipLimiter struct {
	tokens     float64
	lastUpdate time.Time
}

type RateLimiter struct {
	mu    sync.Mutex
	ips   map[string]*ipLimiter
	rps   float64
	burst float64
}

func NewRateLimiter(rps float64, burst float64) *RateLimiter {
	return &RateLimiter{
		ips:   make(map[string]*ipLimiter),
		rps:   rps,
		burst: burst,
	}
}

func (rl *RateLimiter) Limit(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	lim, exists := rl.ips[ip]
	if !exists {
		rl.ips[ip] = &ipLimiter{
			tokens:     rl.burst - 1,
			lastUpdate: time.Now(),
		}
		return false
	}

	now := time.Now()
	elapsed := now.Sub(lim.lastUpdate).Seconds()
	lim.lastUpdate = now

	lim.tokens += elapsed * rl.rps
	if lim.tokens > rl.burst {
		lim.tokens = rl.burst
	}

	if lim.tokens >= 1 {
		lim.tokens -= 1
		return false
	}

	return true
}

func getIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func RateLimit(rps float64, burst float64) func(http.Handler) http.Handler {
	limiter := NewRateLimiter(rps, burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getIP(r)
			if limiter.Limit(ip) {
				http.Error(w, "Too Many Requests - Rate Limit Exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

var (
	AuthLimiter    = NewRateLimiter(10.0/60.0, 10)
	InviteLimiter  = NewRateLimiter(5.0/60.0, 5)
	UploadLimiter  = NewRateLimiter(20.0/60.0, 20)
	GeneralLimiter = NewRateLimiter(120.0/60.0, 120)
)

func DynamicRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getIP(r)
		path := r.URL.Path

		var limiter *RateLimiter
		if strings.HasPrefix(path, "/api/auth/") {
			limiter = AuthLimiter
		} else if path == "/api/invitations/create" {
			limiter = InviteLimiter
		} else if strings.HasPrefix(path, "/api/uploads/") {
			limiter = UploadLimiter
		} else if strings.HasPrefix(path, "/api/") {
			limiter = GeneralLimiter
		}

		if limiter != nil {
			if limiter.Limit(ip) {
				http.Error(w, "Too Many Requests - Rate Limit Exceeded", http.StatusTooManyRequests)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob: http: https:;")
		next.ServeHTTP(w, r)
	})
}

func RequestSizeLimit(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

func DynamicRequestSizeLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var maxBytes int64 = 1024 * 1024 // 1MB
		if strings.HasPrefix(r.URL.Path, "/api/uploads/") {
			maxBytes = 10 * 1024 * 1024 // 10MB
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
}

func IPAllowlist(allowed []string) func(http.Handler) http.Handler {
	allowedMap := make(map[string]bool)
	for _, ip := range allowed {
		allowedMap[ip] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getIP(r)
			if len(allowedMap) > 0 && !allowedMap[ip] {
				http.Error(w, fmt.Sprintf("Forbidden: IP %s is not allowlisted", ip), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
