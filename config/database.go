package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB adalah handle GORM global yang dipakai seluruh repository.
// Skema tidak dikelola lewat AutoMigrate - DDL dimiliki file migrasi SQL tim
// (folder database/), sehingga GORM hanya dipakai sebagai query builder.
var DB *gorm.DB

// ConnectDatabase membuka koneksi PostgreSQL dari variabel environment,
// mengonfigurasi connection pool, lalu memastikan koneksi benar-benar hidup.
func ConnectDatabase() error {
	dbUser := os.Getenv("DB_USER")
	dbName := os.Getenv("DB_NAME")
	if dbUser == "" || dbName == "" {
		return fmt.Errorf("konfigurasi database tidak lengkap: DB_USER dan DB_NAME wajib diisi")
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		env("DB_HOST", "127.0.0.1"),
		dbUser,
		os.Getenv("DB_PASSWORD"),
		dbName,
		env("DB_PORT", "5432"),
		env("DB_SSLMODE", "disable"),
		env("DB_TIMEZONE", "Asia/Jakarta"),
	)

	gormLogger := logger.Default.LogMode(logLevel())

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		return fmt.Errorf("gagal membuka koneksi database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("gagal mengambil sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(envInt("DB_MAX_OPEN_CONNS", 25))
	sqlDB.SetMaxIdleConns(envInt("DB_MAX_IDLE_CONNS", 5))
	sqlDB.SetConnMaxLifetime(time.Duration(envInt("DB_CONN_MAX_LIFETIME_MIN", 30)) * time.Minute)
	sqlDB.SetConnMaxIdleTime(time.Duration(envInt("DB_CONN_MAX_IDLE_MIN", 5)) * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database tidak merespons: %w", err)
	}

	DB = db
	return nil
}

// env mengambil environment dengan fallback bila kosong.
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt membaca integer dari environment dengan fallback bila kosong/invalid.
func envInt(key string, fallback int) int {
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

// logLevel memetakan DB_LOG_LEVEL ke level logger GORM.
func logLevel() logger.LogLevel {
	switch os.Getenv("DB_LOG_LEVEL") {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "info":
		return logger.Info
	default:
		return logger.Warn
	}
}
