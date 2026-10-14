package utils

import (
	"context"
	"errors"

	"google.golang.org/api/idtoken"
)

// ErrGoogleVerify adalah error internal saat verifikasi ID token Google gagal.
var ErrGoogleVerify = errors.New("verifikasi token google gagal")

// GoogleClaims adalah data penting dari ID token Google yang sudah terverifikasi.
type GoogleClaims struct {
	Subject       string
	Email         string
	Name          string
	Picture       string
	EmailVerified bool
}

// GoogleVerifier memverifikasi ID token Google dan mengembalikan klaimnya.
// Diabstraksi sebagai interface agar mudah di-fake pada pengujian.
type GoogleVerifier interface {
	Verify(ctx context.Context, idToken string) (*GoogleClaims, error)
}

// IDTokenVerifier adalah implementasi GoogleVerifier berbasis library resmi.
// Verifikasi mencakup tanda tangan, issuer, audience (client id), dan exp.
type IDTokenVerifier struct {
	clientID string
}

// NewIDTokenVerifier membuat verifier dengan audience (Google client id) tertentu.
func NewIDTokenVerifier(clientID string) *IDTokenVerifier {
	return &IDTokenVerifier{clientID: clientID}
}

// Verify memvalidasi ID token dan mengekstrak klaim yang dibutuhkan.
func (v *IDTokenVerifier) Verify(ctx context.Context, idToken string) (*GoogleClaims, error) {
	payload, err := idtoken.Validate(ctx, idToken, v.clientID)
	if err != nil {
		return nil, ErrGoogleVerify
	}

	return &GoogleClaims{
		Subject:       payload.Subject,
		Email:         stringClaim(payload.Claims, "email"),
		Name:          stringClaim(payload.Claims, "name"),
		Picture:       stringClaim(payload.Claims, "picture"),
		EmailVerified: boolClaim(payload.Claims, "email_verified"),
	}, nil
}

// stringClaim mengambil nilai string dari map klaim.
func stringClaim(claims map[string]interface{}, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	return ""
}

// boolClaim mengambil nilai bool dari map klaim.
func boolClaim(claims map[string]interface{}, key string) bool {
	v, ok := claims[key].(bool)
	return ok && v
}
