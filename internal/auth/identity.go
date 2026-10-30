// Package auth issues and reads pseudonymous browser identity cookies.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
)

// CookieName is the browser identity cookie.
const CookieName = "coverletter_identity"

const (
	tokenBytes = 32
	maxAge     = 10 * 365 * 24 * 60 * 60
)

// NewToken returns a new random token encoded for use as a cookie value.
func NewToken() (string, error) {
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken decodes a cookie token and returns its SHA-256 hash. The boolean
// is false when the token is not a valid identity token.
func HashToken(token string) ([]byte, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != tokenBytes {
		return nil, false
	}
	sum := sha256.Sum256(raw)
	return sum[:], true
}

// Token returns the identity token from the request cookie, if it is a valid
// token.
func Token(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return "", false
	}
	if _, ok := HashToken(cookie.Value); !ok {
		return "", false
	}
	return cookie.Value, true
}

// SetCookie writes the identity cookie. The cookie is Secure when the request
// arrived over TLS or through a proxy that reports https.
func SetCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure(r),
	})
}

func secure(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
