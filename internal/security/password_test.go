package security

import "testing"

func TestPasswordHashAndVerify(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "correct-horse-battery-staple" {
		t.Fatal("password was not hashed")
	}
	if !VerifyPassword("correct-horse-battery-staple", hash) {
		t.Fatal("valid password was rejected")
	}
	if VerifyPassword("wrong-password", hash) {
		t.Fatal("invalid password was accepted")
	}
}
func TestPasswordMinimumLength(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("short password should be rejected")
	}
}
