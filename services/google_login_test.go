package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"e-absensi-backend/utils"
)

// fakeGoogleVerifier mengembalikan klaim tetap untuk pengujian jalur pra-DB.
type fakeGoogleVerifier struct {
	claims *utils.GoogleClaims
	err    error
}

func (f *fakeGoogleVerifier) Verify(ctx context.Context, idToken string) (*utils.GoogleClaims, error) {
	return f.claims, f.err
}

func newGoogleTestService(v utils.GoogleVerifier) *AuthService {
	return &AuthService{google: v, now: time.Now}
}

func TestGoogleLoginDisabledWithoutVerifier(t *testing.T) {
	svc := newGoogleTestService(nil)
	_, err := svc.GoogleLogin(context.Background(), "token", ClientMeta{})
	assert.ErrorIs(t, err, utils.ErrGoogleDisabled)
}

func TestGoogleLoginInvalidToken(t *testing.T) {
	svc := newGoogleTestService(&fakeGoogleVerifier{err: errors.New("signature invalid")})
	_, err := svc.GoogleLogin(context.Background(), "token", ClientMeta{})
	assert.ErrorIs(t, err, utils.ErrGoogleTokenInvalid)
}

func TestGoogleLoginEmailUnverified(t *testing.T) {
	svc := newGoogleTestService(&fakeGoogleVerifier{claims: &utils.GoogleClaims{EmailVerified: false}})
	_, err := svc.GoogleLogin(context.Background(), "token", ClientMeta{})
	assert.ErrorIs(t, err, utils.ErrGoogleEmailUnverified)
}
