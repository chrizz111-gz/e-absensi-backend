package dto

// RegisterStudentRequest adalah payload registrasi mandiri siswa (F1.1).
// Username opsional; NISN wajib 10 digit.
type RegisterStudentRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=150"`
	Email    string `json:"email" binding:"required,email,max=255"`
	NISN     string `json:"nisn" binding:"required,nisn"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Username string `json:"username" binding:"omitempty,min=3,max=50"`
}

// LoginRequest adalah payload login manual (F1.2). Identifier dapat berupa
// email atau username.
type LoginRequest struct {
	Identifier string `json:"identifier" binding:"required"`
	Password   string `json:"password" binding:"required"`
	DeviceInfo string `json:"device_info" binding:"omitempty,max=255"`
}

// RefreshRequest menukar refresh token dengan pasangan token baru.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest mencabut sesi pada device yang mengirim refresh token ini.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UserResponse adalah representasi aman data user (tanpa hash/secret).
type UserResponse struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	Email     string   `json:"email"`
	Username  *string  `json:"username,omitempty"`
	NISN      *string  `json:"nisn,omitempty"`
	AvatarURL *string  `json:"avatar_url,omitempty"`
	IsActive  bool     `json:"is_active"`
	Roles     []string `json:"roles"`
}

// AuthResponse dikembalikan saat login/refresh berhasil.
type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	TokenType    string       `json:"token_type"`
	ExpiresIn    int          `json:"expires_in"`
}
