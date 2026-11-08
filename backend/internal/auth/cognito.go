package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwksCacheDuration   = 6 * time.Hour
	jwksRefreshCooldown = time.Minute
	maxTokenSize        = 16 * 1024
	maxJWKSSize         = 1024 * 1024
)

type CognitoConfig struct {
	Region     string
	UserPoolID string
	ClientID   string
}

type CognitoVerifier struct {
	issuer      string
	clientID    string
	jwksURL     string
	client      *http.Client
	mu          sync.Mutex
	keys        map[string]*rsa.PublicKey
	fetchedAt   time.Time
	attemptedAt time.Time
}

type cognitoClaims struct {
	ClientID string `json:"client_id"`
	TokenUse string `json:"token_use"`
	jwt.RegisteredClaims
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	KeyID     string `json:"kid"`
	KeyType   string `json:"kty"`
	Algorithm string `json:"alg"`
	Use       string `json:"use"`
	Modulus   string `json:"n"`
	Exponent  string `json:"e"`
}

func NewCognitoVerifier(config CognitoConfig) (*CognitoVerifier, error) {
	region := strings.TrimSpace(config.Region)
	userPoolID := strings.TrimSpace(config.UserPoolID)
	clientID := strings.TrimSpace(config.ClientID)
	if region == "" || userPoolID == "" || clientID == "" {
		return nil, errors.New("cognito region, user pool id, and client id are required")
	}

	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, userPoolID)
	return newCognitoVerifier(issuer, clientID, issuer+"/.well-known/jwks.json", &http.Client{
		Timeout: 10 * time.Second,
	}), nil
}

func newCognitoVerifier(issuer string, clientID string, jwksURL string, client *http.Client) *CognitoVerifier {
	return &CognitoVerifier{
		issuer:   issuer,
		clientID: clientID,
		jwksURL:  jwksURL,
		client:   client,
		keys:     make(map[string]*rsa.PublicKey),
	}
}

func (verifier *CognitoVerifier) Verify(ctx context.Context, rawToken string) error {
	_, err := verifier.VerifySubject(ctx, rawToken)
	return err
}

// VerifySubject returns the Cognito user ID only after verifying the access token.
func (verifier *CognitoVerifier) VerifySubject(ctx context.Context, rawToken string) (string, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" || len(rawToken) > maxTokenSize {
		return "", errors.New("invalid access token")
	}

	claims := &cognitoClaims{}
	token, err := jwt.ParseWithClaims(
		rawToken,
		claims,
		func(token *jwt.Token) (any, error) {
			keyID, ok := token.Header["kid"].(string)
			if !ok || keyID == "" {
				return nil, errors.New("access token is missing key id")
			}
			return verifier.key(ctx, keyID)
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithIssuer(verifier.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || !token.Valid {
		return "", errors.New("invalid access token")
	}

	if claims.ClientID != verifier.clientID || claims.TokenUse != "access" || claims.Subject == "" {
		return "", errors.New("invalid access token claims")
	}

	return claims.Subject, nil
}

func (verifier *CognitoVerifier) key(ctx context.Context, keyID string) (*rsa.PublicKey, error) {
	verifier.mu.Lock()
	defer verifier.mu.Unlock()

	key, found := verifier.keys[keyID]
	now := time.Now()
	cacheFresh := now.Sub(verifier.fetchedAt) < jwksCacheDuration
	if found && cacheFresh {
		return key, nil
	}
	if !verifier.attemptedAt.IsZero() && now.Sub(verifier.attemptedAt) < jwksRefreshCooldown {
		if found {
			return key, nil
		}
		return nil, errors.New("access token signing key was not found")
	}

	verifier.attemptedAt = now
	if err := verifier.refreshKeys(ctx); err != nil {
		if found {
			return key, nil
		}
		return nil, err
	}

	key, found = verifier.keys[keyID]
	if !found {
		return nil, errors.New("access token signing key was not found")
	}
	return key, nil
}

func (verifier *CognitoVerifier) refreshKeys(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, verifier.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("create jwks request: %w", err)
	}

	response, err := verifier.client.Do(request)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch jwks: unexpected status %d", response.StatusCode)
	}

	var document jwksDocument
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxJWKSSize))
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, item := range document.Keys {
		key, err := item.publicKey()
		if err != nil {
			continue
		}
		keys[item.KeyID] = key
	}
	if len(keys) == 0 {
		return errors.New("jwks did not contain usable RSA signing keys")
	}

	verifier.keys = keys
	verifier.fetchedAt = time.Now()
	return nil
}

func (item jwk) publicKey() (*rsa.PublicKey, error) {
	if item.KeyID == "" || item.KeyType != "RSA" || item.Use != "sig" {
		return nil, errors.New("unsupported jwk")
	}
	if item.Algorithm != "" && item.Algorithm != jwt.SigningMethodRS256.Alg() {
		return nil, errors.New("unsupported jwk algorithm")
	}

	modulus, err := base64.RawURLEncoding.DecodeString(item.Modulus)
	if err != nil || len(modulus) == 0 {
		return nil, errors.New("invalid jwk modulus")
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(item.Exponent)
	if err != nil || len(exponentBytes) == 0 || len(exponentBytes) > 4 {
		return nil, errors.New("invalid jwk exponent")
	}

	exponent := 0
	for _, value := range exponentBytes {
		exponent = exponent<<8 | int(value)
	}
	if exponent < 3 {
		return nil, errors.New("invalid jwk exponent")
	}

	return &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: exponent}, nil
}
