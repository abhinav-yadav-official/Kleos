package auth

import "testing"

func TestHashPasswordVerifiesOriginalPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	if err := CheckPassword(hash, "correct horse battery staple"); err != nil {
		t.Fatalf("CheckPassword rejected original password: %v", err)
	}
	if err := CheckPassword(hash, "wrong password"); err == nil {
		t.Fatal("CheckPassword accepted wrong password")
	}
}
