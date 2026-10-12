package server

import (
	"net/http/httptest"
	"testing"
)

func TestLocalBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, host, origin, key string
		want                    int
	}{
		{"local", "127.0.0.1:3456", "", "", 200},
		{"IPv6", "[::1]:3456", "", "", 200},
		{"localhost", "localhost:3456", "http://localhost:3000", "", 200},
		{"rebound host", "rebind.example:3456", "", "", 403},
		{"foreign origin", "localhost:3456", "https://foreign.example", "", 403},
		{"unsupported scheme", "localhost", "ftp://localhost", "", 403},
		{"authenticated remote", "gateway.example", "", "local-placeholder", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := New(nil, tc.key, nil)
			r := httptest.NewRequest("GET", "/v1/models", nil)
			r.Host = tc.host
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if tc.key != "" {
				r.Header.Set("Authorization", "Bearer "+tc.key)
			}
			w := httptest.NewRecorder()
			srv.Handler().ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status=%d, want %d", w.Code, tc.want)
			}
		})
	}
}
