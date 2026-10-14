package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType distinguishes access tokens from refresh tokens so that a token
// issued for one purpose can never be replayed for the other.
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

const (
	defaultAccessTTL  = 15 * time.Minute
	defaultRefreshTTL = 7 * 24 * time.Hour
)

var (
	// ErrInvalidToken is returned when a token is malformed, has a bad
	// signature, or is used for the wrong purpose.
	ErrInvalidToken = errors.New("token tidak valid")

	// ErrExpiredToken is returned when a token is well-formed but expired.
	ErrExpiredToken = errors.New("token telah kedaluwarsa")

	// ErrMissingJWTSecret is returned when the JWT secret is not configured.
	ErrMissingJWTSecret = errors.New("konfigurasi JWT_SECRET belum diatur")
)

// Claims is the custom JWT payload embedded in every token issued by the API.
type Claims struct {
	UserID    int64     `json:"uid"`
	Email     string    `json:"email,omitempty"`
	Roles     []string  `json:"roles,omitempty"`
	SessionID string    `json:"sid,omitempty"`
	TokenType TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// accessSecret resolves the HMAC signing secret for access tokens. A dedicated
// secret can be supplied through JWT_SECRET.
func accessSecret() ([]byte, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, ErrMissingJWTSecret
	}
	return []byte(secret), nil
}

// refreshSecret resolves the HMAC signing secret for refresh tokens. For best
// practice a distinct secret should be provided through JWT_REFRESH_SECRET; if
// it is absent the access secret is used as a fallback.
func refreshSecret() ([]byte, error) {
	if secret := os.Getenv("JWT_REFRESH_SECRET"); secret != "" {
		return []byte(secret), nil
	}
	return accessSecret()
}

// durationFromEnv reads a Go duration string (e.g. "15m", "168h") from the
// environment, falling back to a safe default when unset or invalid.
func durationFromEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// GenerateAccessToken issues a short-lived access token (default 15 minutes)
// carrying the user's identity and the union of all roles they own.
func GenerateAccessToken(userID int64, email string, roles []string) (string, error) {
	secret, err := accessSecret()
	if err != nil {
		return "", err
	}
	return signToken(secret, Claims{
		UserID:    userID,
		Email:     email,
		Roles:     roles,
		TokenType: AccessToken,
	}, durationFromEnv("JWT_ACCESS_TTL", defaultAccessTTL))
}

// GenerateRefreshToken issues a long-lived refresh token (default 7 days) bound
// to a session identifier. Only the hash of the resulting token is persisted
// in the sessions table.
func GenerateRefreshToken(userID int64, sessionID string) (string, error) {
	secret, err := refreshSecret()
	if err != nil {
		return "", err
	}
	return signToken(secret, Claims{
		UserID:    userID,
		SessionID: sessionID,
		TokenType: RefreshToken,
	}, durationFromEnv("JWT_REFRESH_TTL", defaultRefreshTTL))
}

// AccessTokenTTL mengembalikan masa berlaku access token.
func AccessTokenTTL() time.Duration {
	return durationFromEnv("JWT_ACCESS_TTL", defaultAccessTTL)
}

// RefreshTokenTTL mengembalikan masa berlaku refresh token.
func RefreshTokenTTL() time.Duration {
	return durationFromEnv("JWT_REFRESH_TTL", defaultRefreshTTL)
}

// signToken builds and signs a JWT using HS256.
func signToken(secret []byte, claims Claims, ttl time.Duration) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("gagal menandatangani token: %w", err)
	}
	return signed, nil
}

// ParseToken validates a token's signature and expiration, then verifies that
// its type matches the expected one before returning the embedded claims.
func ParseToken(tokenString string, expected TokenType) (*Claims, error) {
	secret, err := secretForType(expected)
	if err != nil {
		return nil, err
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrInvalidToken
			}
			return secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !token.Valid || claims.TokenType != expected {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ParseAccessToken is a convenience wrapper around ParseToken.
func ParseAccessToken(tokenString string) (*Claims, error) {
	return ParseToken(tokenString, AccessToken)
}

// ParseRefreshToken is a convenience wrapper around ParseToken.
func ParseRefreshToken(tokenString string) (*Claims, error) {
	return ParseToken(tokenString, RefreshToken)
}

func secretForType(tokenType TokenType) ([]byte, error) {
	if tokenType == RefreshToken {
		return refreshSecret()
	}
	return accessSecret()
}

// HashToken returns the hex-encoded SHA-256 digest of a token. High-entropy
// values such as refresh tokens are stored as digests (not bcrypt) so lookups
// stay fast while a database leak still exposes no usable token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
