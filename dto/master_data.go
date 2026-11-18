package dto

import "time"

// =============================================================================
// F3.5 — Tahun Ajaran (Academic Years)
// =============================================================================

// AcademicYearListFilter adalah filter tervalidasi untuk GET /academic-years.
type AcademicYearListFilter struct {
	Page           int
	Limit          int
	Q              string
	IsActive       *bool
	ActiveSemester string
}

// AcademicYearResponse adalah representasi tahun ajaran untuk API.
type AcademicYearResponse struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	ActiveSemester string    `json:"active_semester"`
	StartDate      string    `json:"start_date"`
	EndDate        *string   `json:"end_date"`
	IsActive       bool      `json:"is_active"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AcademicYearListResponse adalah payload daftar tahun ajaran berhalaman.
type AcademicYearListResponse struct {
	Items []AcademicYearResponse `json:"items"`
	Meta  PaginationResponse     `json:"meta"`
}

// CreateAcademicYearRequest adalah payload POST /academic-years.
type CreateAcademicYearRequest struct {
	Name           string  `json:"name" binding:"required,max=20"`
	ActiveSemester string  `json:"active_semester" binding:"required"`
	StartDate      string  `json:"start_date" binding:"required"`
	EndDate        *string `json:"end_date"`
	IsActive       *bool   `json:"is_active"`
}

// UpdateAcademicYearRequest adalah payload PATCH /academic-years/{id}.
type UpdateAcademicYearRequest struct {
	Name           *string    `json:"name"`
	ActiveSemester *string    `json:"active_semester"`
	StartDate      *string    `json:"start_date"`
	EndDate        *string    `json:"end_date"`
	ClearEndDate   bool       `json:"clear_end_date"`
	IsActive       *bool      `json:"is_active"`
	UpdatedAt      *time.Time `json:"updated_at" binding:"required"`
}

// =============================================================================
// F3.4 — Mata Pelajaran (Subjects)
// =============================================================================

// SubjectListFilter adalah filter tervalidasi untuk GET /subjects.
type SubjectListFilter struct {
	Page     int
	Limit    int
	Q        string
	IsActive *bool
}

// SubjectResponse adalah representasi mata pelajaran untuk API.
type SubjectResponse struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SubjectListResponse adalah payload daftar mata pelajaran berhalaman.
type SubjectListResponse struct {
	Items []SubjectResponse  `json:"items"`
	Meta  PaginationResponse `json:"meta"`
}

// CreateSubjectRequest adalah payload POST /subjects.
type CreateSubjectRequest struct {
	Code     string `json:"code" binding:"required,max=20"`
	Name     string `json:"name" binding:"required,max=100"`
	IsActive *bool  `json:"is_active"`
}

// UpdateSubjectRequest adalah payload PATCH /subjects/{id}.
type UpdateSubjectRequest struct {
	Code      *string    `json:"code"`
	Name      *string    `json:"name"`
	IsActive  *bool      `json:"is_active"`
	UpdatedAt *time.Time `json:"updated_at" binding:"required"`
}

// =============================================================================
// F3.3 — Kelas (Classes)
// =============================================================================

// ClassListFilter adalah filter tervalidasi untuk GET /classes.
type ClassListFilter struct {
	Page           int
	Limit          int
	Q              string
	AcademicYearID *int64
	GradeLevel     *int16
	IsActive       *bool
}

// ClassAcademicYearSummary adalah ringkasan tahun ajaran pada response kelas.
type ClassAcademicYearSummary struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	ActiveSemester string `json:"active_semester"`
	IsActive       bool   `json:"is_active"`
}

// ClassWaliKelasSummary adalah ringkasan akun wali kelas pada response kelas.
type ClassWaliKelasSummary struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ClassStudentSummary adalah ringkasan anggota siswa pada detail kelas.
type ClassStudentSummary struct {
	ID       int64  `json:"id"`
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	NISN     string `json:"nisn"`
	IsActive bool   `json:"is_active"`
}

// ClassResponse adalah representasi kelas untuk daftar maupun detail.
// Students hanya diisi pada GET /classes/{id} jika role pemanggil berhak
// melihat anggota siswa kelas tersebut.
type ClassResponse struct {
	ID              int64                     `json:"id"`
	Name            string                    `json:"name"`
	GradeLevel      int16                     `json:"grade_level"`
	AcademicYearID  int64                     `json:"academic_year_id"`
	AcademicYear    *ClassAcademicYearSummary `json:"academic_year,omitempty"`
	WaliKelasUserID *int64                    `json:"wali_kelas_user_id"`
	WaliKelas       *ClassWaliKelasSummary    `json:"wali_kelas,omitempty"`
	IsActive        bool                      `json:"is_active"`
	StudentCount    int64                     `json:"student_count"`
	Students        []ClassStudentSummary     `json:"students,omitempty"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

// ClassListResponse adalah payload daftar kelas berhalaman.
type ClassListResponse struct {
	Items []ClassResponse    `json:"items"`
	Meta  PaginationResponse `json:"meta"`
}

// CreateClassRequest adalah payload POST /classes.
type CreateClassRequest struct {
	Name           string `json:"name" binding:"required,max=50"`
	GradeLevel     int16  `json:"grade_level" binding:"required"`
	AcademicYearID int64  `json:"academic_year_id" binding:"required"`
	IsActive       *bool  `json:"is_active"`
}

// UpdateClassRequest adalah payload PATCH /classes/{id}.
type UpdateClassRequest struct {
	Name           *string    `json:"name"`
	GradeLevel     *int16     `json:"grade_level"`
	AcademicYearID *int64     `json:"academic_year_id"`
	IsActive       *bool      `json:"is_active"`
	UpdatedAt      *time.Time `json:"updated_at" binding:"required"`
}

// =============================================================================
// F3.2 — Data Guru (Teachers)
// =============================================================================

// TeacherListFilter adalah filter tervalidasi untuk GET /teachers.
type TeacherListFilter struct {
	Page      int
	Limit     int
	Q         string
	SubjectID *int64
	IsActive  *bool
}

// TeacherCandidateResponse adalah akun ber-role guru_mapel yang belum punya
// profil guru (untuk dropdown pada form tambah guru).
type TeacherCandidateResponse struct {
	UserID    int64   `json:"user_id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url,omitempty"`
	IsActive  bool    `json:"is_active"`
}

// TeacherSubjectSummary adalah ringkasan mata pelajaran yang diampu guru.
type TeacherSubjectSummary struct {
	ID       int64  `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// TeacherResponse adalah representasi profil guru beserta akun dan mapelnya.
type TeacherResponse struct {
	ID        int64                   `json:"id"`
	UserID    int64                   `json:"user_id"`
	Name      string                  `json:"name"`
	Email     string                  `json:"email"`
	AvatarURL *string                 `json:"avatar_url,omitempty"`
	IsActive  bool                    `json:"is_active"`
	NIP       string                  `json:"nip"`
	Subjects  []TeacherSubjectSummary `json:"subjects"`
	UpdatedAt time.Time               `json:"updated_at"`
}

// TeacherListResponse adalah payload daftar guru berhalaman.
type TeacherListResponse struct {
	Items []TeacherResponse  `json:"items"`
	Meta  PaginationResponse `json:"meta"`
}

// CreateTeacherRequest adalah payload POST /teachers.
type CreateTeacherRequest struct {
	UserID     int64   `json:"user_id" binding:"required"`
	NIP        string  `json:"nip" binding:"required,max=30"`
	SubjectIDs []int64 `json:"subject_ids" binding:"required"`
}

// UpdateTeacherRequest adalah payload PATCH /teachers/{id}.
type UpdateTeacherRequest struct {
	NIP        *string    `json:"nip"`
	SubjectIDs *[]int64   `json:"subject_ids"`
	UpdatedAt  *time.Time `json:"updated_at" binding:"required"`
}

// =============================================================================
// F3.1 — Data Siswa (Students)
// =============================================================================

// StudentListFilter adalah filter tervalidasi untuk GET /students.
type StudentListFilter struct {
	Page            int
	Limit           int
	Q               string
	ClassID         *int64
	Unassigned      bool
	IsActive        *bool
	IncludeArchived bool
}

// StudentClassSummary adalah ringkasan kelas siswa saat ini.
type StudentClassSummary struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	GradeLevel     int16  `json:"grade_level"`
	AcademicYearID int64  `json:"academic_year_id"`
	IsActive       bool   `json:"is_active"`
}

// StudentResponse adalah representasi profil akademik siswa beserta akunnya.
type StudentResponse struct {
	ID             int64                `json:"id"`
	UserID         int64                `json:"user_id"`
	Name           string               `json:"name"`
	Email          string               `json:"email"`
	AvatarURL      *string              `json:"avatar_url,omitempty"`
	NISN           string               `json:"nisn"`
	ClassID        *int64               `json:"class_id"`
	Class          *StudentClassSummary `json:"class,omitempty"`
	IsActive       bool                 `json:"is_active"`
	InactiveReason *string              `json:"inactive_reason,omitempty"`
	DeactivatedAt  *time.Time           `json:"deactivated_at,omitempty"`
	ArchivedAt     *time.Time           `json:"archived_at,omitempty"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

// StudentListResponse adalah payload daftar siswa berhalaman.
type StudentListResponse struct {
	Items []StudentResponse  `json:"items"`
	Meta  PaginationResponse `json:"meta"`
}

// UpdateStudentRequest adalah payload PATCH /students/{id}.
// Hanya menerima whitelist: name, nisn, class_id (dan unassign_class bila
// ingin mengosongkan class_id), serta updated_at wajib untuk optimistic locking.
type UpdateStudentRequest struct {
	Name          *string    `json:"name"`
	NISN          *string    `json:"nisn"`
	ClassID       *int64     `json:"class_id"`
	UnassignClass bool       `json:"unassign_class"`
	UpdatedAt     *time.Time `json:"updated_at" binding:"required"`
}
