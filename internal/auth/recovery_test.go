package auth

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestAuthManager_RefreshRecovery(t *testing.T) {
	db := setupTestDBWithCreds(t, "rejected", "refresh-0", time.Now().Add(time.Hour).Unix(), "us-east-1")
	defer db.Close()
	calls := 0
	srv := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.UnmarshalRead(r.Body, &body); err != nil {
			t.Error(err)
			return
		}
		if body["refreshToken"] != fmt.Sprintf("refresh-%d", calls) {
			t.Error("stale refresh token reused")
			w.WriteHeader(401)
			return
		}
		calls++
		_ = json.MarshalWrite(w, map[string]any{"accessToken": fmt.Sprintf("access-%d", calls), "refreshToken": fmt.Sprintf("refresh-%d", calls), "expiresIn": 3600})
	}))
	defer srv.Close()
	mgr := newAuthManagerWithDB(db)
	mgr.oidcEndpointFn = func(string) string { return srv.URL }
	if _, err := mgr.GetToken(t.Context()); err != nil {
		t.Fatal(err)
	}
	mgr.InvalidateCache()
	first, err := mgr.GetToken(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if first.AccessToken != "access-1" || calls != 1 {
		t.Fatal("403 recovery did not refresh an unexpired token")
	}
	mgr.mu.Lock()
	mgr.cached.ExpiresAt = 0
	mgr.mu.Unlock()
	second, err := mgr.GetToken(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if second.AccessToken != "access-2" || calls != 2 {
		t.Fatal("rotated credentials were not retained")
	}
}

func TestAuthManager_StaticKeyDoesNotRefresh(t *testing.T) {
	mgr := NewAuthManager("missing", WithAPIKey("ksk_placeholder", ""))
	mgr.InvalidateCache()
	creds, err := mgr.GetToken(t.Context())
	if err != nil || creds.AccessToken != "ksk_placeholder" {
		t.Fatal("static key unexpectedly refreshed")
	}
}
