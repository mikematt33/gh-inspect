package github

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestPoolBestSelectsHighestQuota(t *testing.T) {
	pool := NewPool(
		NewStaticTokenSource("a", "tok-a"),
		NewStaticTokenSource("b", "tok-b"),
	)
	// Default remaining is 5000 for both; make tok-b higher.
	pool.creds[0].remaining = 100
	pool.creds[1].remaining = 4000

	best := pool.Best()
	if best == nil || best.Name() != "tok-b" {
		t.Fatalf("expected tok-b, got %v", best)
	}
}

func TestPoolExhaustionAndRotation(t *testing.T) {
	pool := NewPool(
		NewStaticTokenSource("a", "tok-a"),
		NewStaticTokenSource("b", "tok-b"),
	)
	reset := time.Now().Add(time.Hour)
	pool.creds[0].markExhausted(reset)

	best := pool.Best()
	if best == nil || best.Name() != "tok-b" {
		t.Fatalf("expected rotation to tok-b, got %v", best)
	}

	// Exhaust the second one too; no credential should remain.
	pool.creds[1].markExhausted(reset)
	if pool.Best() != nil {
		t.Fatal("expected no usable credential")
	}
	if _, ok := pool.EarliestReset(); !ok {
		t.Fatal("expected an earliest reset time")
	}
}

func TestPoolResetReplenishes(t *testing.T) {
	pool := NewPool(NewStaticTokenSource("a", "tok-a"))
	// Exhausted but reset already passed -> should be considered available again.
	pool.creds[0].markExhausted(time.Now().Add(-time.Minute))
	if got := pool.creds[0].available(); got <= 0 {
		t.Fatalf("expected replenished quota, got %d", got)
	}
}

func TestPoolTotalAvailableAndSummaries(t *testing.T) {
	pool := NewPool(
		NewStaticTokenSource("a", "tok-a"),
		NewStaticTokenSource("b", "tok-b"),
	)
	pool.creds[0].remaining = 1000
	pool.creds[1].remaining = 2000
	if got := pool.TotalAvailable(); got != 3000 {
		t.Errorf("total available = %d, want 3000", got)
	}
	sums := pool.Summaries()
	if len(sums) != 2 {
		t.Fatalf("summaries = %d, want 2", len(sums))
	}
	// Sorted by remaining descending.
	if sums[0].Remaining < sums[1].Remaining {
		t.Error("summaries not sorted by remaining descending")
	}
}

func TestSignAndDecodeAppJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	token, err := signAppJWT(123456, key)
	if err != nil {
		t.Fatalf("signAppJWT: %v", err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt should have 3 parts, got %d", len(parts))
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if claims["iss"] != "123456" {
		t.Errorf("iss = %v, want 123456", claims["iss"])
	}
	iat, _ := claims["iat"].(float64)
	exp, _ := claims["exp"].(float64)
	if exp <= iat {
		t.Errorf("exp (%v) should be after iat (%v)", exp, iat)
	}
}

func TestParseRSAPrivateKeyPKCS1AndPKCS8(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	pkcs1 := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if _, err := parseRSAPrivateKey(pkcs1); err != nil {
		t.Errorf("PKCS1 parse failed: %v", err)
	}

	der, _ := x509.MarshalPKCS8PrivateKey(key)
	pkcs8 := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if _, err := parseRSAPrivateKey(pkcs8); err != nil {
		t.Errorf("PKCS8 parse failed: %v", err)
	}

	if _, err := parseRSAPrivateKey([]byte("not a key")); err == nil {
		t.Error("expected error for invalid key")
	}
}

func TestAppTokenSourceCachesAndRefreshes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pkcs1 := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	src, err := NewAppTokenSource(AppCredentials{
		Name:           "test-app",
		AppID:          1,
		InstallationID: 2,
		PrivateKeyPEM:  pkcs1,
	})
	if err != nil {
		t.Fatalf("NewAppTokenSource: %v", err)
	}
	if src.Kind() != "app" {
		t.Errorf("kind = %q, want app", src.Kind())
	}

	// Inject a cached token directly to verify caching avoids network calls.
	impl := src.(*appInstallationSource)
	impl.cached = "ghs_cached"
	impl.expires = time.Now().Add(30 * time.Minute)

	tok, err := impl.Token(context.Background())
	if err != nil {
		t.Fatalf("Token: %v", err)
	}
	if tok != "ghs_cached" {
		t.Errorf("token = %q, want cached", tok)
	}
}
