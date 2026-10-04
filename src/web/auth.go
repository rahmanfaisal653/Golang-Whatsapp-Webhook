package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	keysPath      = "src/session/keys.json"
	sessionCookie = "gowa_admin"
	sessionTTL    = 24 * time.Hour
)

// auth guards the admin API with a password-based session cookie and stores
// the API keys used to authenticate /notify callers.
type auth struct {
	email    string
	password string

	mu       sync.Mutex
	sessions map[string]time.Time
}

func newAuth() *auth {
	email := os.Getenv("ADMIN_EMAIL")
	if email == "" {
		email = "admin@gmail.com"
	}
	pw := os.Getenv("ADMIN_PASSWORD")
	if pw == "" {
		pw = randomToken(6)
		log.Printf("web: ADMIN_PASSWORD not set, generated admin password: %s", pw)
	}
	return &auth{email: email, password: pw, sessions: make(map[string]time.Time)}
}

// require wraps a handler with admin-session authentication.
func (a *auth) require(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.validSession(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (a *auth) validSession(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	exp, ok := a.sessions[c.Value]
	if !ok || time.Now().After(exp) {
		delete(a.sessions, c.Value)
		return false
	}
	return true
}

func (a *auth) checkCredentials(email, pw string) bool {
	emailOK := subtle.ConstantTimeCompare([]byte(email), []byte(a.email)) == 1
	pwOK := subtle.ConstantTimeCompare([]byte(pw), []byte(a.password)) == 1
	return emailOK && pwOK
}

func (a *auth) newSession() string {
	tok := randomToken(32)
	a.mu.Lock()
	a.sessions[tok] = time.Now().Add(sessionTTL)
	a.mu.Unlock()
	return tok
}

func (a *auth) dropSession(r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		a.mu.Unlock()
	}
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// APIKey is one stored /notify credential. Only the hash is persisted.
type APIKey struct {
	ID      string    `json:"id"`
	Label   string    `json:"label"`
	Hash    string    `json:"hash"`
	Created time.Time `json:"created"`
}

var keysMu sync.Mutex

func loadKeys() ([]APIKey, error) {
	keysMu.Lock()
	defer keysMu.Unlock()
	return readKeys()
}

func readKeys() ([]APIKey, error) {
	b, err := os.ReadFile(keysPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var keys []APIKey
	if err := json.Unmarshal(b, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

func writeKeys(keys []APIKey) error {
	if err := os.MkdirAll(filepath.Dir(keysPath), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(keysPath, b, 0o600)
}

func hashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// createKey adds a key and returns its plaintext, which is shown only once.
func createKey(label string) (string, APIKey, error) {
	keysMu.Lock()
	defer keysMu.Unlock()
	keys, err := readKeys()
	if err != nil {
		return "", APIKey{}, err
	}
	plain := randomToken(24)
	k := APIKey{ID: randomToken(6), Label: label, Hash: hashKey(plain), Created: time.Now()}
	keys = append(keys, k)
	if err := writeKeys(keys); err != nil {
		return "", APIKey{}, err
	}
	return plain, k, nil
}

func deleteKey(id string) error {
	keysMu.Lock()
	defer keysMu.Unlock()
	keys, err := readKeys()
	if err != nil {
		return err
	}
	out := keys[:0]
	for _, k := range keys {
		if k.ID != id {
			out = append(out, k)
		}
	}
	return writeKeys(out)
}

// verifyKey reports whether key matches a stored key.
func verifyKey(key string) bool {
	if key == "" {
		return false
	}
	keys, err := loadKeys()
	if err != nil {
		return false
	}
	h := hashKey(key)
	for _, k := range keys {
		if subtle.ConstantTimeCompare([]byte(k.Hash), []byte(h)) == 1 {
			return true
		}
	}
	return false
}
