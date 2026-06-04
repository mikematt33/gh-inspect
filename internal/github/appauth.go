package github

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
)

// TokenSource produces a usable GitHub access token on demand.
// Implementations may return a static PAT or a short-lived GitHub App
// installation token that is transparently refreshed before expiry.
type TokenSource interface {
	// Token returns a valid access token, refreshing it if necessary.
	Token(ctx context.Context) (string, error)
	// Name is a human-friendly identifier used in reporting.
	Name() string
	// Kind reports the credential type ("pat" or "app").
	Kind() string
}

// staticToken is a TokenSource backed by a fixed personal access token.
type staticToken struct {
	token string
	name  string
}

// NewStaticTokenSource wraps a PAT as a TokenSource.
func NewStaticTokenSource(token, name string) TokenSource {
	return &staticToken{token: strings.TrimSpace(token), name: name}
}

func (s *staticToken) Token(_ context.Context) (string, error) {
	if s.token == "" {
		return "", fmt.Errorf("empty token")
	}
	return s.token, nil
}

func (s *staticToken) Name() string { return s.name }
func (s *staticToken) Kind() string { return "pat" }

// AppCredentials holds the configuration required to authenticate as a
// GitHub App installation.
type AppCredentials struct {
	Name           string // optional friendly name
	AppID          int64
	InstallationID int64
	PrivateKeyPEM  []byte // PEM-encoded RSA private key
}

// appInstallationSource implements TokenSource for a GitHub App installation.
// It generates a JWT from the App private key, exchanges it for a short-lived
// installation access token, and caches that token until shortly before expiry.
type appInstallationSource struct {
	creds        AppCredentials
	key          *rsa.PrivateKey
	name         string
	newJWTClient func(jwt string) *github.Client

	mu      sync.Mutex
	cached  string
	expires time.Time
}

// NewAppTokenSource constructs a TokenSource that authenticates as a GitHub App
// installation. The private key must be a PEM-encoded RSA key.
func NewAppTokenSource(creds AppCredentials) (TokenSource, error) {
	key, err := parseRSAPrivateKey(creds.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parsing app private key: %w", err)
	}
	name := creds.Name
	if name == "" {
		name = fmt.Sprintf("app:%d/install:%d", creds.AppID, creds.InstallationID)
	}
	return &appInstallationSource{
		creds: creds,
		key:   key,
		name:  name,
		newJWTClient: func(jwt string) *github.Client {
			return github.NewClient(nil).WithAuthToken(jwt)
		},
	}, nil
}

func (a *appInstallationSource) Name() string { return a.name }
func (a *appInstallationSource) Kind() string { return "app" }

// Token returns a cached installation token, refreshing it when it is within
// five minutes of expiry. Installation tokens are valid for one hour.
func (a *appInstallationSource) Token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cached != "" && time.Until(a.expires) > 5*time.Minute {
		return a.cached, nil
	}

	jwt, err := signAppJWT(a.creds.AppID, a.key)
	if err != nil {
		return "", fmt.Errorf("signing app JWT: %w", err)
	}

	client := a.newJWTClient(jwt)
	tok, _, err := client.Apps.CreateInstallationToken(ctx, a.creds.InstallationID, nil)
	if err != nil {
		return "", fmt.Errorf("creating installation token: %w", err)
	}

	a.cached = tok.GetToken()
	a.expires = tok.GetExpiresAt().Time
	if a.expires.IsZero() {
		// Default GitHub installation token lifetime is one hour.
		a.expires = time.Now().Add(time.Hour)
	}
	if a.cached == "" {
		return "", fmt.Errorf("received empty installation token")
	}
	return a.cached, nil
}

// parseRSAPrivateKey decodes a PEM-encoded RSA private key in either PKCS#1 or
// PKCS#8 format.
func parseRSAPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in private key")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unsupported private key format: %w", err)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key is not an RSA key")
	}
	return rsaKey, nil
}

// signAppJWT builds and signs a GitHub App JWT (RS256) using the App ID as the
// issuer. GitHub requires the token to be valid for at most 10 minutes; this
// uses a 9-minute window with a 60-second backdated issued-at to tolerate clock
// drift.
func signAppJWT(appID int64, key *rsa.PrivateKey) (string, error) {
	now := time.Now()
	header := map[string]string{"alg": "RS256", "typ": "JWT"}
	claims := map[string]interface{}{
		"iat": now.Add(-60 * time.Second).Unix(),
		"exp": now.Add(9 * time.Minute).Unix(),
		"iss": fmt.Sprintf("%d", appID),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	signingInput := base64URL(headerJSON) + "." + base64URL(claimsJSON)
	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signingInput + "." + base64URL(sig), nil
}

func base64URL(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// LoadAppPrivateKey resolves an App private key from either a PEM file path or
// an inline PEM string (e.g. provided via an environment variable).
func LoadAppPrivateKey(pathOrPEM string) ([]byte, error) {
	trimmed := strings.TrimSpace(pathOrPEM)
	if strings.Contains(trimmed, "-----BEGIN") {
		return []byte(trimmed), nil
	}
	data, err := os.ReadFile(trimmed)
	if err != nil {
		return nil, fmt.Errorf("reading app private key file: %w", err)
	}
	return data, nil
}
