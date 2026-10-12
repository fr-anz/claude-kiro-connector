package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// WithCredentialCache enables a gateway-owned token cache. The Kiro DB remains read-only.
func WithCredentialCache(path string) Option {
	return func(m *AuthManager) { m.cachePath = path }
}

type credentialCache struct {
	SourceID     string `json:"source_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
	ProfileARN   string `json:"profile_arn,omitempty"`
}

func (m *AuthManager) loadCredentialCache(source *Credentials) *Credentials {
	hash := sha256.Sum256([]byte(m.dbPath + "\x00" + source.RefreshToken + "\x00" + source.ClientID + "\x00" + source.ProfileARN))
	m.sourceID = hex.EncodeToString(hash[:])
	if m.cachePath == "" {
		return source
	}
	f, err := os.Open(m.cachePath)
	if err != nil {
		return source
	}
	defer f.Close()
	var saved credentialCache
	if err := json.UnmarshalRead(io.LimitReader(f, 64<<10), &saved); err != nil || saved.SourceID != m.sourceID || saved.AccessToken == "" || saved.RefreshToken == "" {
		return source
	}
	result := *source
	result.AccessToken, result.RefreshToken, result.ExpiresAt = saved.AccessToken, saved.RefreshToken, saved.ExpiresAt
	if saved.ProfileARN != "" {
		result.ProfileARN = saved.ProfileARN
	}
	return &result
}

func (m *AuthManager) saveCredentialCache(creds *Credentials) error {
	if m.cachePath == "" {
		return nil
	}
	dir := filepath.Dir(m.cachePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".auth-cache-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := f.Chmod(0600); err != nil {
		return err
	}
	saved := credentialCache{SourceID: m.sourceID, AccessToken: creds.AccessToken, RefreshToken: creds.RefreshToken, ExpiresAt: creds.ExpiresAt, ProfileARN: creds.ProfileARN}
	if err := json.MarshalWrite(f, saved); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), m.cachePath)
}

func (m *AuthManager) persistCredentials(creds *Credentials) {
	if err := m.saveCredentialCache(creds); err != nil {
		slog.Warn("refreshed credentials could not be persisted; restart recovery unavailable", "err", err)
	}
}
