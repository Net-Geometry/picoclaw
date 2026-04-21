package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/fileutil"
)

type AuthCredential struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	AccountID    string    `json:"account_id,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Provider     string    `json:"provider"`
	AuthMethod   string    `json:"auth_method"`
	Email        string    `json:"email,omitempty"`
	ProjectID    string    `json:"project_id,omitempty"`
}

type AuthStore struct {
	// Credentials is the legacy single-credential map (kept for backward compatibility).
	Credentials map[string]*AuthCredential `json:"credentials,omitempty"`
	// CredentialList stores multiple credentials per provider.
	CredentialList map[string][]*AuthCredential `json:"credential_list,omitempty"`
}

func (c *AuthCredential) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

func (c *AuthCredential) NeedsRefresh() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(5 * time.Minute).After(c.ExpiresAt)
}

func authFilePath() string {
	return filepath.Join(config.GetHome(), "auth.json")
}

func LoadStore() (*AuthStore, error) {
	path := authFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AuthStore{
				Credentials:    make(map[string]*AuthCredential),
				CredentialList: make(map[string][]*AuthCredential),
			}, nil
		}
		return nil, err
	}

	var store AuthStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	if store.Credentials == nil {
		store.Credentials = make(map[string]*AuthCredential)
	}
	if store.CredentialList == nil {
		store.CredentialList = make(map[string][]*AuthCredential)
	}
	// Migrate legacy single-credential entries into multi-credential buckets.
	for provider, cred := range store.Credentials {
		if cred == nil {
			continue
		}
		if len(store.CredentialList[provider]) == 0 {
			store.CredentialList[provider] = []*AuthCredential{cred}
		}
	}
	return &store, nil
}

func SaveStore(store *AuthStore) error {
	path := authFilePath()
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}

	// Use unified atomic write utility with explicit sync for flash storage reliability.
	return fileutil.WriteFileAtomic(path, data, 0o600)
}

func GetCredential(provider string) (*AuthCredential, error) {
	store, err := LoadStore()
	if err != nil {
		return nil, err
	}
	if creds := store.CredentialList[provider]; len(creds) > 0 {
		for i := len(creds) - 1; i >= 0; i-- {
			if creds[i] != nil {
				return creds[i], nil
			}
		}
	}
	cred, ok := store.Credentials[provider]
	if !ok {
		return nil, nil
	}
	return cred, nil
}

func SetCredential(provider string, cred *AuthCredential) error {
	store, err := LoadStore()
	if err != nil {
		return err
	}
	store.CredentialList[provider] = upsertCredential(store.CredentialList[provider], cred)
	store.Credentials[provider] = cred
	return SaveStore(store)
}

// ListCredentials returns all stored credentials for a provider.
func ListCredentials(provider string) ([]*AuthCredential, error) {
	store, err := LoadStore()
	if err != nil {
		return nil, err
	}
	creds := store.CredentialList[provider]
	if len(creds) == 0 {
		if legacy, ok := store.Credentials[provider]; ok && legacy != nil {
			return []*AuthCredential{legacy}, nil
		}
		return nil, nil
	}
	out := make([]*AuthCredential, 0, len(creds))
	for _, c := range creds {
		if c != nil {
			out = append(out, c)
		}
	}
	return out, nil
}

func DeleteCredential(provider string) error {
	store, err := LoadStore()
	if err != nil {
		return err
	}
	delete(store.Credentials, provider)
	delete(store.CredentialList, provider)
	return SaveStore(store)
}

func DeleteAllCredentials() error {
	path := authFilePath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func upsertCredential(existing []*AuthCredential, next *AuthCredential) []*AuthCredential {
	if next == nil {
		return existing
	}
	id := credentialIdentity(next)
	for i, c := range existing {
		if c == nil {
			continue
		}
		if credentialIdentity(c) == id {
			existing[i] = next
			return existing
		}
	}
	return append(existing, next)
}

func credentialIdentity(c *AuthCredential) string {
	if c == nil {
		return ""
	}
	if v := strings.TrimSpace(c.AccountID); v != "" {
		return "account:" + v
	}
	if v := strings.TrimSpace(c.Email); v != "" {
		return "email:" + strings.ToLower(v)
	}
	if v := strings.TrimSpace(c.AccessToken); v != "" {
		if len(v) > 24 {
			v = v[:24]
		}
		return "token:" + v
	}
	return ""
}
