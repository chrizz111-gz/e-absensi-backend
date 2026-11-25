package dto

import "time"

// =============================================================================
// EPIC 5 — Student Attendance
// =============================================================================

// AttendanceFormStudentItem adalah satu baris siswa pada form ceklis absensi (F5.1 & F5.2).
type AttendanceFormStudentItem struct {
	StudentID              int64      `json:"student_id"`
	UserID                 int64      `json:"user_id"`
	Name                   string     `json:"name"`
	NISN                   string     `json:"nisn"`
	AttendanceID           *int64     `json:"attendance_id,omitempty"`
	Status                 string     `json:"status"`
	Source                 *string    `json:"source,omitempty"`
	Note                   *string    `json:"note,omitempty"`
	IsLockedByLeaveRequest bool       `json:"is_locked_by_leave_request"`
	LeaveRequestID         *int64     `json:"leave_request_id,omitempty"`
	LeaveRequestStatus     *string    `json:"leave_request_status,omitempty"`
	UpdatedAt              *time.Time `json:"updated_at,omitempty"`
}

// ScheduleAttendanceFormResponse adalah payload GET /schedules/{id}/attendance-form (F5.1 & F5.2).
type ScheduleAttendanceFormResponse struct {
	Schedule          ScheduleResponse            `json:"schedule"`
	TeacherAttendance *TeacherAttendanceResponse  `json:"teacher_attendance,omitempty"`
	FormMode          string                      `json:"form_mode"`
	IsAlreadyRecorded bool                        `json:"is_already_recorded"`
	CanSubmit         bool                        `json:"can_submit"`
	Students          []AttendanceFormStudentItem `json:"students"`
}

// SubmitStudentAttendanceItem adalah satu item input status absensi siswa pada POST /schedules/{id}/attendances.
type SubmitStudentAttendanceItem struct {
	StudentID int64   `json:"student_id" binding:"required"`
	Status    string  `json:"status" binding:"required"`
	Note      *string `json:"note"`
}

// SubmitScheduleAttendancesRequest adalah payload POST /schedules/{id}/attendances (F5.1 & F5.2).
type SubmitScheduleAttendancesRequest struct {
	Items []SubmitStudentAttendanceItem `json:"items" binding:"required"`
}

// StudentSummary adalah ringkasan siswa pada data absensi/izin/surat.
type StudentSummary struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	NISN     string `json:"nisn"`
	ClassID  *int64 `json:"class_id,omitempty"`
	IsActive bool   `json:"is_active"`
}

// StudentAttendanceResponse adalah representasi satu baris absensi siswa (F5.1-F5.4).
type StudentAttendanceResponse struct {
	ID             int64                 `json:"id"`
	ScheduleID     int64                 `json:"schedule_id"`
	Schedule       *ScheduleResponse     `json:"schedule,omitempty"`
	StudentID      int64                 `json:"student_id"`
	Student        *StudentSummary       `json:"student,omitempty"`
	ClassID        int64                 `json:"class_id"`
	Class          *ScheduleClassSummary `json:"class,omitempty"`
	AcademicYearID int64                 `json:"academic_year_id"`
	Status         string                `json:"status"`
	Source         string                `json:"source"`
	RecordedBy     int64                 `json:"recorded_by"`
	Note           *string               `json:"note"`
	Version        int                   `json:"version"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

// SubmitScheduleAttendancesResponse adalah hasil POST /schedules/{id}/attendances (F5.1 & F5.2).
type SubmitScheduleAttendancesResponse struct {
	ScheduleID    int64                       `json:"schedule_id"`
	Source        string                      `json:"source"`
	RecordedCount int                         `json:"recorded_count"`
	Items         []StudentAttendanceResponse `json:"items"`
}

// UpdateStudentAttendanceRequest adalah payload PATCH /attendances/{id} (F5.3).
type UpdateStudentAttendanceRequest struct {
	Status    string     `json:"status" binding:"required"`
	Note      *string    `json:"note"`
	UpdatedAt *time.Time `json:"updated_at" binding:"required"`
}

// StudentAttendanceListFilter adalah parameter query untuk GET /attendances (F5.4).
type StudentAttendanceListFilter struct {
	Page       int
	Limit      int
	Date       string
	DateFrom   string
	DateTo     string
	Preset     string
	ClassID    *int64
	ScheduleID *int64
	StudentID  *int64
	SubjectID  *int64
	Status     string
	Source     string
}

// StudentAttendanceSummaryStats adalah ringkasan statistik jumlah status absensi siswa (F5.4).
type StudentAttendanceSummaryStats struct {
	Total      int64 `json:"total"`
	Hadir      int64 `json:"hadir"`
	Izin       int64 `json:"izin"`
	Sakit      int64 `json:"sakit"`
	Alpha      int64 `json:"alpha"`
	Dispensasi int64 `json:"dispensasi"`
}

// StudentAttendanceListResponse adalah hasil berhalaman GET /attendances (F5.4).
type StudentAttendanceListResponse struct {
	Items    []StudentAttendanceResponse   `json:"items"`
	Summary  StudentAttendanceSummaryStats `json:"summary"`
	DateFrom string                        `json:"date_from"`
	DateTo   string                        `json:"date_to"`
	Meta     PaginationResponse            `json:"meta"`
}

// StudentAttendanceRecapItem adalah ringkasan rekap absensi per siswa (F5.5 & F5.10).
type StudentAttendanceRecapItem struct {
	StudentID                  int64   `json:"student_id"`
	UserID                     int64   `json:"user_id"`
	StudentName                string  `json:"student_name"`
	NISN                       string  `json:"nisn"`
	ClassID                    int64   `json:"class_id"`
	ClassName                  string  `json:"class_name"`
	HadirSessions              int64   `json:"hadir_sessions"`
	IzinSessions               int64   `json:"izin_sessions"`
	SakitSessions              int64   `json:"sakit_sessions"`
	AlphaSessions              int64   `json:"alpha_sessions"`
	DispensasiSessions         int64   `json:"dispensasi_sessions"`
	TotalSessions              int64   `json:"total_sessions"`
	AttendanceRatePercent      float64 `json:"attendance_rate_percent"`
	AlphaDaysInRange           int64   `json:"alpha_days_in_range"`
	TotalAlphaDaysAcademicYear int64   `json:"total_alpha_days_academic_year"`
	LatestWarningThreshold     *int16  `json:"latest_warning_threshold"`
}

// StudentAttendanceRecapResponse adalah hasil GET /attendances/recap (F5.5).
type StudentAttendanceRecapResponse struct {
	Items    []StudentAttendanceRecapItem  `json:"items"`
	Summary  StudentAttendanceSummaryStats `json:"summary"`
	DateFrom string                        `json:"date_from"`
	DateTo   string                        `json:"date_to"`
	Meta     PaginationResponse            `json:"meta"`
}

// SubmitLeaveRequestPayload adalah payload untuk POST /leave-requests (F5.6).
type SubmitLeaveRequestPayload struct {
	Type            string `json:"type" form:"type"`
	Date            string `json:"date" form:"date"`
	Reason          string `json:"reason" form:"reason"`
	EvidenceFileURL string `json:"evidence_file_url" form:"evidence_file_url"`
}

// ReviewLeaveRequestPayload adalah payload opsional untuk approve/reject izin/sakit (F5.7).
type ReviewLeaveRequestPayload struct {
	Note string `json:"note"`
}

// LeaveRequestResponse adalah representasi pengajuan izin/sakit siswa (F5.6, F5.7, F5.9).
// Untuk role yang tidak berhak melihat alasan & file bukti pribadi (Guru Mapel & Siswa Inti
// pada tampilan kelas F5.9), Reason dan EvidenceFileURL dikosongkan (omitempty).
type LeaveRequestResponse struct {
	ID              int64                 `json:"id"`
	StudentID       int64                 `json:"student_id"`
	Student         *StudentSummary       `json:"student,omitempty"`
	ClassID         int64                 `json:"class_id"`
	Class           *ScheduleClassSummary `json:"class,omitempty"`
	Type            string                `json:"type"`
	Date            string                `json:"date"`
	Reason          string                `json:"reason,omitempty"`
	EvidenceFileURL string                `json:"evidence_file_url,omitempty"`
	Status          string                `json:"status"`
	ReviewedBy      *int64                `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time            `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
}

// LeaveRequestListFilter adalah parameter filter untuk GET /leave-requests (F5.6/F5.7/F5.9).
type LeaveRequestListFilter struct {
	Page      int
	Limit     int
	Date      string
	DateFrom  string
	DateTo    string
	ClassID   *int64
	StudentID *int64
	Type      string
	Status    string
}

// LeaveRequestListResponse adalah daftar pengajuan izin/sakit berhalaman.
type LeaveRequestListResponse struct {
	Items []LeaveRequestResponse `json:"items"`
	Meta  PaginationResponse     `json:"meta"`
}

// ClassTodayLeaveSummaryResponse adalah hasil GET /leave-requests/class-today (F5.9).
type ClassTodayLeaveSummaryResponse struct {
	Date       string                 `json:"date"`
	TotalItems int                    `json:"total_items"`
	TotalDraft int                    `json:"total_draft"`
	TotalIzin  int                    `json:"total_izin"`
	TotalSakit int                    `json:"total_sakit"`
	Items      []LeaveRequestResponse `json:"items"`
}

// UploadPermissionLetterPayload adalah payload POST /permission-letters (F5.8).
type UploadPermissionLetterPayload struct {
	StudentID       int64   `json:"student_id" form:"student_id"`
	Type            string  `json:"type" form:"type"`
	EvidenceFileURL string  `json:"evidence_file_url" form:"evidence_file_url"`
	Note            *string `json:"note" form:"note"`
}

// PermissionLetterResponse adalah representasi surat izin tatib / dispensasi (F5.8).
type PermissionLetterResponse struct {
	ID              int64           `json:"id"`
	StudentID       int64           `json:"student_id"`
	Student         *StudentSummary `json:"student,omitempty"`
	Type            string          `json:"type"`
	UploadedBy      int64           `json:"uploaded_by"`
	UploaderName    string          `json:"uploader_name,omitempty"`
	EvidenceFileURL string          `json:"evidence_file_url"`
	Note            *string         `json:"note"`
	CreatedAt       time.Time       `json:"created_at"`
}

// PermissionLetterListFilter adalah filter untuk GET /permission-letters (F5.8).
type PermissionLetterListFilter struct {
	Page      int
	Limit     int
	DateFrom  string
	DateTo    string
	ClassID   *int64
	StudentID *int64
	Type      string
}

// PermissionLetterListResponse adalah daftar surat izin berhalaman (F5.8).
type PermissionLetterListResponse struct {
	Items []PermissionLetterResponse `json:"items"`
	Meta  PaginationResponse         `json:"meta"`
}

// AlphaWarningItem adalah satu catatan peringatan dini akumulasi Alpha (F5.10).
type AlphaWarningItem struct {
	ID             int64           `json:"id"`
	StudentID      int64           `json:"student_id"`
	Student        *StudentSummary `json:"student,omitempty"`
	AcademicYearID int64           `json:"academic_year_id"`
	Threshold      int16           `json:"threshold"`
	AlphaDays      int16           `json:"alpha_days"`
	TriggeredAt    time.Time       `json:"triggered_at"`
}

// AlphaWarningListFilter adalah filter untuk GET /attendances/alpha-warnings (F5.10).
type AlphaWarningListFilter struct {
	Page           int
	Limit          int
	AcademicYearID *int64
	ClassID        *int64
	StudentID      *int64
	Threshold      *int16
}

// AlphaWarningListResponse adalah daftar log peringatan dini Alpha berhalaman (F5.10).
type AlphaWarningListResponse struct {
	Items []AlphaWarningItem `json:"items"`
	Meta  PaginationResponse `json:"meta"`
}
