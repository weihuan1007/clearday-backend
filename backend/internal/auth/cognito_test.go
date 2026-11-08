package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestCognitoVerifierAcceptsValidAccessToken(t *testing.T) {
	fixture := newVerifierFixture(t)
	token := fixture.sign(t, "client-1", "access", fixture.issuer, time.Now().Add(time.Hour))

	if err := fixture.verifier.Verify(context.Background(), token); err != nil {
		t.Fatalf("verify valid access token: %v", err)
	}
}

func TestCognitoVerifierRejectsWrongClient(t *testing.T) {
	fixture := newVerifierFixture(t)
	token := fixture.sign(t, "another-client", "access", fixture.issuer, time.Now().Add(time.Hour))

	if err := fixture.verifier.Verify(context.Background(), token); err == nil {
		t.Fatal("expected wrong client token to be rejected")
	}
}

func TestCognitoVerifierRejectsIDToken(t *testing.T) {
	fixture := newVerifierFixture(t)
	token := fixture.sign(t, "client-1", "id", fixture.issuer, time.Now().Add(time.Hour))

	if err := fixture.verifier.Verify(context.Background(), token); err == nil {
		t.Fatal("expected id token to be rejected")
	}
}

func TestCognitoVerifierRejectsExpiredToken(t *testing.T) {
	fixture := newVerifierFixture(t)
	token := fixture.sign(t, "client-1", "access", fixture.issuer, time.Now().Add(-time.Hour))

	if err := fixture.verifier.Verify(context.Background(), token); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

func TestCognitoVerifierThrottlesRefreshForUnknownKey(t *testing.T) {
	fixture := newVerifierFixture(t)
	validToken := fixture.sign(t, "client-1", "access", fixture.issuer, time.Now().Add(time.Hour))
	if err := fixture.verifier.Verify(context.Background(), validToken); err != nil {
		t.Fatalf("prime signing key cache: %v", err)
	}

	unknownKeyToken := fixture.signWithKeyID(t, "unknown-key")
	_ = fixture.verifier.Verify(context.Background(), unknownKeyToken)
	_ = fixture.verifier.Verify(context.Background(), unknownKeyToken)

	if requests := fixture.requests.Load(); requests != 1 {
		t.Fatalf("JWKS requests = %d, want 1", requests)
	}
}

type verifierFixture struct {
	privateKey *rsa.PrivateKey
	issuer     string
	server     *httptest.Server
	verifier   *CognitoVerifier
	requests   *atomic.Int32
}

func newVerifierFixture(t *testing.T) verifierFixture {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	keyID := "test-key"
	document := jwksDocument{Keys: []jwk{{
		KeyID:     keyID,
		KeyType:   "RSA",
		Algorithm: "RS256",
		Use:       "sig",
		Modulus:   base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes()),
		Exponent:  base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes()),
	}}}
	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("marshal JWKS: %v", err)
	}

	requests := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write(payload)
	}))
	t.Cleanup(server.Close)

	issuer := "https://cognito-idp.ap-southeast-1.amazonaws.com/ap-southeast-1_example"
	return verifierFixture{
		privateKey: privateKey,
		issuer:     issuer,
		server:     server,
		verifier:   newCognitoVerifier(issuer, "client-1", server.URL, server.Client()),
		requests:   requests,
	}
}

func (fixture verifierFixture) signWithKeyID(t *testing.T, keyID string) string {
	t.Helper()

	now := time.Now()
	claims := cognitoClaims{
		ClientID: "client-1",
		TokenUse: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    fixture.issuer,
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = keyID
	signed, err := token.SignedString(fixture.privateKey)
	if err != nil {
		t.Fatalf("sign access token: %v", err)
	}
	return signed
}

func (fixture verifierFixture) sign(t *testing.T, clientID string, tokenUse string, issuer string, expires time.Time) string {
	t.Helper()

	now := time.Now()
	claims := cognitoClaims{
		ClientID: clientID,
		TokenUse: tokenUse,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(expires),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-key"
	signed, err := token.SignedString(fixture.privateKey)
	if err != nil {
		t.Fatalf("sign access token: %v", err)
	}
	return signed
}
