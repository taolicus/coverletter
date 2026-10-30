package auth

import (
	"testing"
)

func TestNewTokenIsUniqueAndHashable(t *testing.T) {
	first, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("two tokens are equal")
	}

	firstHash, ok := HashToken(first)
	if !ok {
		t.Fatal("new token is not valid")
	}
	secondHash, ok := HashToken(second)
	if !ok {
		t.Fatal("new token is not valid")
	}
	if len(firstHash) != 32 {
		t.Errorf("hash length = %d, want 32", len(firstHash))
	}
	if string(firstHash) == string(secondHash) {
		t.Error("different tokens have the same hash")
	}
}

func TestHashTokenRejectsMalformedTokens(t *testing.T) {
	for _, token := range []string{"", "not base64!", "c2hvcnQ"} {
		if _, ok := HashToken(token); ok {
			t.Errorf("HashToken(%q) accepted an invalid token", token)
		}
	}
}
