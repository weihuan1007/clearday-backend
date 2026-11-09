package auth

import (
	"context"
	"strings"
	"testing"
)

func TestPersonalMoneyVerifier(t *testing.T) {
	key := strings.Repeat("a1", 32)
	for _, invalidKey := range []string{"", "short", strings.Repeat("zz", 32)} {
		if NewPersonalMoneyVerifier(invalidKey) != nil {
			t.Fatal("invalid Money key enabled the ledger")
		}
	}
	verify := NewPersonalMoneyVerifier(key)
	if verify == nil {
		t.Fatal("valid key did not enable the ledger")
	}
	for _, wrongToken := range []string{"", "old-reminder-token", strings.Repeat("b2", 32)} {
		if owner, err := verify(context.Background(), wrongToken); owner != "" || err == nil {
			t.Fatal("incorrect key opened the ledger")
		}
	}
	owner, err := verify(context.Background(), key)
	if err != nil || owner != "personal" {
		t.Fatalf("valid key got owner %q and error %v", owner, err)
	}
}
