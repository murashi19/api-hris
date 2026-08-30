package security

import (
	"testing"
	"time"
)

func TestAccessTokenRoundTrip(t *testing.T) {
	secret := "a-secret-that-is-at-least-32-bytes-long"
	raw, err := CreateAccessToken(42, secret, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	id, err := ParseAccessToken(raw, secret)
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 {
		t.Fatalf("want 42, got %d", id)
	}
}
func TestOpaqueTokenHashesDiffer(t *testing.T) {
	rawA, hashA, err := NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	rawB, hashB, err := NewOpaqueToken()
	if err != nil {
		t.Fatal(err)
	}
	if rawA == rawB || hashA == hashB {
		t.Fatal("tokens must be unique")
	}
	if HashToken(rawA) != hashA {
		t.Fatal("token hash is not deterministic")
	}
}
