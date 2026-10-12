package auth

import (
	"encoding/json/v2"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
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
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	mgr := NewAuthManager("missing", WithAPIKey("ksk_placeholder", ""), WithCredentialCache(cachePath))
	mgr.InvalidateCache()
	creds, err := mgr.GetToken(t.Context())
	if err != nil || creds.AccessToken != "ksk_placeholder" {
		t.Fatal("static key unexpectedly refreshed")
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatal("static key was persisted")
	}
}

func TestAuthManager_RefreshRestart(t *testing.T) {
	db := setupTestDBWithCreds(t, "original", "refresh-original", time.Now().Add(time.Hour).Unix(), "us-east-1")
	defer db.Close()
	calls := 0
	srv := newTCP4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.UnmarshalRead(r.Body, &body); err != nil {
			t.Error(err)
			return
		}
		if calls == 1 && body["refreshToken"] != "refresh-rotated" {
			t.Error("restart lost rotated refresh token")
			w.WriteHeader(401)
			return
		}
		calls++
		_ = json.MarshalWrite(w, map[string]any{"accessToken": "rotated", "refreshToken": "refresh-rotated", "expiresIn": 3600})
	}))
	defer srv.Close()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	newManager := func() *AuthManager {
		mgr := newAuthManagerWithDB(db)
		WithCredentialCache(cachePath)(mgr)
		mgr.oidcEndpointFn = func(string) string { return srv.URL }
		return mgr
	}
	mgr := newManager()
	mgr.InvalidateCache()
	if _, err := mgr.GetToken(t.Context()); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0600 {
		t.Fatal("cache is not owner-only")
	}
	restarted := newManager()
	creds, err := restarted.GetToken(t.Context())
	if err != nil || creds.AccessToken != "rotated" || calls != 1 {
		t.Fatal("restart did not restore cached access token")
	}
	restarted.InvalidateCache()
	if _, err := restarted.GetToken(t.Context()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("forced refresh after restart was skipped")
	}
	original, err := ReadCredentials(db)
	if err != nil || original.AccessToken != "original" {
		t.Fatal("Kiro DB changed during cache persistence")
	}
	newLogin, _ := json.Marshal(map[string]any{"accessToken": "new-login", "refreshToken": "new-login-refresh", "expiresAt": time.Now().Add(time.Hour).Unix()})
	if _, err := db.Exec(`UPDATE auth_kv SET value=? WHERE key='kirocli:odic:token'`, string(newLogin)); err != nil {
		t.Fatal(err)
	}
	creds, err = newManager().GetToken(t.Context())
	if err != nil || creds.AccessToken != "new-login" {
		t.Fatal("old cache overrides a new Kiro login")
	}
}
