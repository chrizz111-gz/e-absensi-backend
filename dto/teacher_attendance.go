package dto

import "time"

// =============================================================================
// EPIC 6 — Teacher Attendance & QR
// =============================================================================

// GenerateTeacherQRRequest adalah payload opsional untuk POST /teacher-attendance/qr/generate (F6.1).
// Jika kosong, sistem otomatis mendeteksi jadwal pelajaran aktif pada kelas user.
type GenerateTeacherQRRequest struct {
	ScheduleID *int64 `json:"schedule_id"`
	ClassID    *int64 `json:"class_id"`
}

// GenerateTeacherQRResponse adalah payload hasil generate QR & kode cadangan (F6.1).
type GenerateTeacherQRResponse struct {
	ScheduleID        int64            `json:"schedule_id"`
	QRToken           string           `json:"qr_token"`
	BackupCode        string           `json:"backup_code"`
	ExpiresAt         time.Time        `json:"expires_at"`
	ExpiresInSeconds  int              `json:"expires_in_seconds"`
	GeneratedByUserID int64            `json:"generated_by_user_id"`
	GeneratedByRole   string           `json:"generated_by_role"`
	Schedule          ScheduleResponse `json:"schedule"`
}

// ConfirmTeacherAttendanceRequest adalah payload POST /teacher-attendance/confirm (F6.2).
// Guru dapat mengirimkan qr_token (hasil scan kamera) atau backup_code (6 karakter).
type ConfirmTeacherAttendanceRequest struct {
	QRToken    string `json:"qr_token"`
	BackupCode string `json:"backup_code"`
}

// TeacherAttendanceUserSummary adalah ringkasan user pada data kehadiran guru.
type TeacherAttendanceUserSummary struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Role string `json:"role,omitempty"`
}

// TeacherAttendanceResponse adalah representasi lengkap satu baris kehadiran guru (F6.2/F6.4).
type TeacherAttendanceResponse struct {
	ID                     int64                         `json:"id"`
	ScheduleID             int64                         `json:"schedule_id"`
	Schedule               *ScheduleResponse             `json:"schedule,omitempty"`
	Status                 string                        `json:"status"`
	ScheduledTeacher       *ScheduleTeacherSummary       `json:"scheduled_teacher,omitempty"`
	ConfirmedTeacherID     *int64                        `json:"confirmed_teacher_id"`
	ConfirmedTeacher       *ScheduleTeacherSummary       `json:"confirmed_teacher,omitempty"`
	IsSubstitute           bool                          `json:"is_substitute"`
	ConfirmationMethod     *string                       `json:"confirmation_method"`
	ConfirmedAt            *time.Time                    `json:"confirmed_at"`
	GeneratedByUserID      *int64                        `json:"generated_by_user_id"`
	GeneratedByRole        *string                       `json:"generated_by_role"`
	GeneratedBy            *TeacherAttendanceUserSummary `json:"generated_by,omitempty"`
	Keterangan             *string                       `json:"keterangan"`
	KeteranganUpdatedBy    *int64                        `json:"keterangan_updated_by"`
	AttendanceFormUnlocked bool                          `json:"attendance_form_unlocked"`
	CreatedAt              time.Time                     `json:"created_at"`
	UpdatedAt              time.Time                     `json:"updated_at"`
}

// TeacherAttendanceListFilter adalah parameter filter untuk GET /teacher-attendance & export (F6.4/F6.5).
type TeacherAttendanceListFilter struct {
	Page      int
	Limit     int
	DateFrom  string
	DateTo    string
	Preset    string
	TeacherID *int64
	ClassID   *int64
	Status    string
}

// TeacherAttendanceSummaryStats adalah statistik ringkasan kehadiran guru (F6.4).
type TeacherAttendanceSummaryStats struct {
	TotalSessions         int64   `json:"total_sessions"`
	TotalHadir            int64   `json:"total_hadir"`
	TotalGuruPengganti    int64   `json:"total_guru_pengganti"`
	TotalTidakHadir       int64   `json:"total_tidak_hadir"`
	AttendanceRatePercent float64 `json:"attendance_rate_percent"`
}

// TeacherAttendanceListResponse adalah hasil berhalaman GET /teacher-attendance (F6.4).
type TeacherAttendanceListResponse struct {
	Items    []TeacherAttendanceResponse   `json:"items"`
	Summary  TeacherAttendanceSummaryStats `json:"summary"`
	DateFrom string                        `json:"date_from"`
	DateTo   string                        `json:"date_to"`
	Meta     PaginationResponse            `json:"meta"`
}

// AnnotateTeacherAttendanceRequest adalah payload PATCH /teacher-attendance/{id}/keterangan (F6.4).
type AnnotateTeacherAttendanceRequest struct {
	Keterangan *string    `json:"keterangan"`
	UpdatedAt  *time.Time `json:"updated_at" binding:"required"`
}

// CorrectTeacherAttendanceRequest adalah payload PATCH /teacher-attendance/{id}/correct (F6.4).
type CorrectTeacherAttendanceRequest struct {
	Status             string     `json:"status" binding:"required"`
	ConfirmedTeacherID *int64     `json:"confirmed_teacher_id"`
	Reason             string     `json:"reason"`
	Keterangan         *string    `json:"keterangan"`
	UpdatedAt          *time.Time `json:"updated_at" binding:"required"`
}
