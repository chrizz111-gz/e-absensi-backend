package dto

import "time"

// =============================================================================
// EPIC 4 — Schedule Management
// =============================================================================

// ScheduleListFilter adalah parameter tervalidasi untuk GET /schedules.
type ScheduleListFilter struct {
	Page      int
	Limit     int
	DateFrom  string
	DateTo    string
	ClassID   *int64
	TeacherID *int64
	SubjectID *int64
	Type      string
}

// ScheduleClassSummary adalah ringkasan kelas pada jadwal.
type ScheduleClassSummary struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	GradeLevel int16  `json:"grade_level"`
	IsActive   bool   `json:"is_active"`
}

// ScheduleSubjectSummary adalah ringkasan mata pelajaran pada jadwal.
type ScheduleSubjectSummary struct {
	ID       int64  `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// ScheduleTeacherSummary adalah ringkasan guru pengampu pada jadwal.
type ScheduleTeacherSummary struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	NIP      string `json:"nip"`
	IsActive bool   `json:"is_active"`
}

// ScheduleResponse adalah representasi satu baris jadwal.
// Ketika Type == "kegiatan", SubjectID dan Subject dikembalikan nil sesuai
// kontrak PRD F4.6, sementara teacher_id tetap dipertahankan.
type ScheduleResponse struct {
	ID           int64                   `json:"id"`
	Date         string                  `json:"date"`
	StartTime    string                  `json:"start_time"`
	EndTime      string                  `json:"end_time"`
	ClassID      int64                   `json:"class_id"`
	Class        *ScheduleClassSummary   `json:"class,omitempty"`
	SubjectID    *int64                  `json:"subject_id"`
	Subject      *ScheduleSubjectSummary `json:"subject,omitempty"`
	TeacherID    int64                   `json:"teacher_id"`
	Teacher      *ScheduleTeacherSummary `json:"teacher,omitempty"`
	Type         string                  `json:"type"`
	ActivityName *string                 `json:"activity_name"`
	CreatedBy    *int64                  `json:"created_by,omitempty"`
	Version      int                     `json:"version"`
	UpdatedAt    time.Time               `json:"updated_at"`
}

// ScheduleListResponse adalah payload daftar jadwal berhalaman.
type ScheduleListResponse struct {
	Items    []ScheduleResponse `json:"items"`
	DateFrom string             `json:"date_from"`
	DateTo   string             `json:"date_to"`
	Meta     PaginationResponse `json:"meta"`
}

// ScheduleImportRowPreview adalah hasil validasi satu baris Excel pada preview import.
type ScheduleImportRowPreview struct {
	RowNumber    int      `json:"row_number"`
	Date         string   `json:"date"`
	StartTime    string   `json:"start_time"`
	EndTime      string   `json:"end_time"`
	ClassName    string   `json:"class_name"`
	SubjectCode  string   `json:"subject_code"`
	TeacherEmail string   `json:"teacher_email"`
	ClassID      int64    `json:"class_id,omitempty"`
	SubjectID    int64    `json:"subject_id,omitempty"`
	TeacherID    int64    `json:"teacher_id,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	Errors       []string `json:"errors,omitempty"`
}

// ScheduleImportPreviewResponse adalah hasil POST /schedules/import/preview.
type ScheduleImportPreviewResponse struct {
	ImportToken  string                     `json:"import_token"`
	ExpiresAt    time.Time                  `json:"expires_at"`
	TotalRows    int                        `json:"total_rows"`
	ValidCount   int                        `json:"valid_count"`
	SkippedCount int                        `json:"skipped_count"`
	ErrorCount   int                        `json:"error_count"`
	ValidRows    []ScheduleImportRowPreview `json:"valid_rows"`
	SkippedRows  []ScheduleImportRowPreview `json:"skipped_rows"`
	ErrorRows    []ScheduleImportRowPreview `json:"error_rows"`
}

// ScheduleImportConfirmRequest adalah payload POST /schedules/import/confirm.
type ScheduleImportConfirmRequest struct {
	ImportToken string `json:"import_token" binding:"required"`
}

// ScheduleImportConfirmResponse adalah hasil POST /schedules/import/confirm.
type ScheduleImportConfirmResponse struct {
	ImportedCount int64 `json:"imported_count"`
	SkippedCount  int64 `json:"skipped_count"`
}

// UpdateScheduleRequest adalah payload PATCH /schedules/{id}.
type UpdateScheduleRequest struct {
	Date      *string    `json:"date"`
	StartTime *string    `json:"start_time"`
	EndTime   *string    `json:"end_time"`
	ClassID   *int64     `json:"class_id"`
	SubjectID *int64     `json:"subject_id"`
	TeacherID *int64     `json:"teacher_id"`
	UpdatedAt *time.Time `json:"updated_at" binding:"required"`
}

// ConvertScheduleToActivityRequest adalah payload PATCH /schedules/{id}/convert-to-activity.
type ConvertScheduleToActivityRequest struct {
	ActivityName string     `json:"activity_name" binding:"required,max=150"`
	UpdatedAt    *time.Time `json:"updated_at" binding:"required"`
}

// RevertScheduleToLessonRequest adalah payload PATCH /schedules/{id}/revert-to-lesson.
type RevertScheduleToLessonRequest struct {
	SubjectID *int64     `json:"subject_id"`
	UpdatedAt *time.Time `json:"updated_at" binding:"required"`
}
