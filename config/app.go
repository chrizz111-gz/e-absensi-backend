package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Konfigurasi kebijakan keamanan Epic 1. Nilai default mengikuti PRD; setiap
// nilai dapat ditimpa lewat environment. Diisi oleh Init() yang dipanggil
// setelah godotenv.Load() pada main, agar .env sudah terbaca.
var (
	// Lockout: 5 gagal login berturut-turut DALAM window 15 menit -> kunci 15 menit.
	MaxFailedLoginAttempts int
	LoginAttemptWindow     time.Duration
	AccountLockDuration    time.Duration

	// OTP: 8 karakter, berlaku 5 menit, resend setelah 60 detik, maks 5 resend/jam,
	// maks 5 percobaan salah sebelum kode hangus.
	OTPTTL              time.Duration
	OTPResendCooldown   time.Duration
	OTPMaxResendPerHour int
	OTPMaxAttempts      int

	// Rate limit per-IP untuk endpoint sensitif (login/register/OTP/Google).
	RateLimitRPS   float64
	RateLimitBurst int
)

// Init memuat seluruh konfigurasi dari environment. Wajib dipanggil setelah
// godotenv.Load() dan sebelum service digunakan.
func Init() {
	MaxFailedLoginAttempts = envIntDefault("MAX_FAILED_LOGIN_ATTEMPTS", 5)
	LoginAttemptWindow = envDurationDefault("LOGIN_ATTEMPT_WINDOW", 15*time.Minute)
	AccountLockDuration = envDurationDefault("ACCOUNT_LOCK_DURATION", 15*time.Minute)

	OTPTTL = envDurationDefault("OTP_TTL", 5*time.Minute)
	OTPResendCooldown = envDurationDefault("OTP_RESEND_COOLDOWN", 60*time.Second)
	OTPMaxResendPerHour = envIntDefault("OTP_MAX_RESEND_PER_HOUR", 5)
	OTPMaxAttempts = envIntDefault("OTP_MAX_ATTEMPTS", 5)

	RateLimitRPS = envFloatDefault("RATE_LIMIT_RPS", 5)
	RateLimitBurst = envIntDefault("RATE_LIMIT_BURST", 10)
}

// GoogleClientID mengembalikan OAuth Client ID Google (audience ID token).
// Kosong berarti fitur Google Sign-In dinonaktifkan.
func GoogleClientID() string {
	return env("GOOGLE_CLIENT_ID", "")
}

// CORSAllowedOrigins mengembalikan daftar origin yang diizinkan mengakses API.
// Dibaca dari CORS_ALLOWED_ORIGINS (dipisah koma). Default: origin dev umum
// untuk React (CRA :3000 dan Vite :5173).
func CORSAllowedOrigins() []string {
	raw := env("CORS_ALLOWED_ORIGINS", "")

	if raw == "" {
		return []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:5173",
		}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}

// AppEnv mengembalikan mode aplikasi (development/production).
func AppEnv() string {
	return env("APP_ENV", "development")
}

// IsProduction mengembalikan true bila aplikasi berjalan di mode production.
func IsProduction() bool {
	return AppEnv() == "production"
}

// AppPort mengembalikan port HTTP server.
func AppPort() string {
	return env("APP_PORT", "8080")
}

// envIntDefault membaca integer dari environment dengan fallback.
func envIntDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}

// envDurationDefault membaca durasi (mis. "15m") dengan fallback.
func envDurationDefault(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(v)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// envFloatDefault membaca float dari environment dengan fallback.
func envFloatDefault(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
