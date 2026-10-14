package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"e-absensi-backend/config"
	"e-absensi-backend/routes"
	"e-absensi-backend/utils"
)

func main() {
	// 1. Muat variabel dari .env
	if err := godotenv.Load(); err != nil {
		log.Println("Peringatan: File .env tidak ditemukan, menggunakan variabel sistem default")
	}

	// 2. Muat konfigurasi (kebijakan lockout/OTP, mode app).
	config.Init()

	// 3. Hubungkan ke database PostgreSQL.
	if err := config.ConnectDatabase(); err != nil {
		log.Fatalf("Gagal terhubung ke database: %v", err)
	}
	log.Println("Berhasil terhubung ke database PostgreSQL")

	// 4. Daftarkan validator kustom (nisn, dll).
	utils.RegisterValidators()

	// 5. Siapkan router.
	if config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	// 6. Pasang route.
	routes.RegisterRoutes(router, config.DB)

	// 7. Jalankan server.
	addr := ":" + config.AppPort()
	log.Printf("Server berjalan di http://localhost%s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Server gagal dijalankan: %v", err)
	}
}
