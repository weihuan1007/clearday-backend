package auth

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
)

// NewPersonalMoneyVerifier protects a single ledger with a separately generated
// key. The stable owner lets the key be rotated without moving the records.
func NewPersonalMoneyVerifier(key string) func(context.Context, string) (string, error) {
	decoded, err := hex.DecodeString(key)
	if err != nil || len(decoded) != 32 {
		return nil
	}
	return func(_ context.Context, token string) (string, error) {
		if subtle.ConstantTimeCompare([]byte(token), []byte(key)) != 1 {
			return "", errors.New("invalid Money key")
		}
		return "personal", nil
	}
}
