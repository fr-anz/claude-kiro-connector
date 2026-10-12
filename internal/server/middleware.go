package server

import (
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/fr-anz/claude-kiro-connector/internal/httpx"
	"github.com/fr-anz/claude-kiro-connector/internal/logging"
	"github.com/fr-anz/claude-kiro-connector/internal/tracing"
)

func traceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := tracing.ExtractTraceID(r.Context())
		if traceID == "" {
			traceID = logging.NewTraceID()
		}
		ctx := logging.WithTraceID(r.Context(), traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authMiddleware enforces API key authentication when configured.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.apiKey == "" {
			u, err := url.Parse("//" + r.Host)
			if err != nil || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || !isLoopbackHost(u.Hostname()) {
				httpx.WriteError(w, http.StatusForbidden, httpx.ErrTypeInvalidRequest, "unauthenticated gateway requires a loopback Host")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(authHeader, "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.apiKey)) != 1 {
			httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrTypeAuthentication, "invalid API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers for localhost origins.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && !isLocalhostOrigin(origin) {
			httpx.WriteError(w, http.StatusForbidden, httpx.ErrTypeInvalidRequest, "browser origin must be localhost")
			return
		}
		if isLocalhostOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isLocalhostOrigin checks if the origin is a localhost URL using strict URL parsing.
func isLocalhostOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	// Reject origins with userinfo or path components that could be spoofed.
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	return isLoopbackHost(u.Hostname())
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
