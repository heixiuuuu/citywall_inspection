package authn

import (
	"testing"
	"time"
)

func TestSignAndParse(t *testing.T) {
	tm := NewTokenManager("test-secret", time.Hour)
	token, err := tm.Sign(42, []string{"INSPECTOR", "ADMIN"})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := tm.Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("user id = %d, want 42", claims.UserID)
	}
	if len(claims.Roles) != 2 || claims.Roles[0] != "INSPECTOR" {
		t.Fatalf("roles = %v", claims.Roles)
	}
}

func TestParseInvalid(t *testing.T) {
	tm := NewTokenManager("test-secret", time.Hour)
	if _, err := tm.Parse("not-a-token"); err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestParseWrongSecret(t *testing.T) {
	tm1 := NewTokenManager("secret-a", time.Hour)
	tm2 := NewTokenManager("secret-b", time.Hour)
	token, err := tm1.Sign(1, nil)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := tm2.Parse(token); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}
