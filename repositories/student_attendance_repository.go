package repositories

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/models"
)

// StudentAttendanceListParams adalah parameter tervalidasi untuk daftar dan rekap absensi siswa.
type StudentAttendanceListParams struct {
	Page               int
	Limit              int
	DateFrom           time.Time
	DateTo             time.Time
	ClassID            *int64
	ScheduleID         *int64
	StudentID          *int64
	SubjectID          *int64
	AcademicYearID     *int64
	Status             string
	Source             string
	RestrictToClassIDs []int64
	RestrictToTeacher  *int64
	RestrictToStudent  *int64
	DenyAll            bool
}

// StudentAttendanceStatusCounts menyimpan agregasi jumlah per status absensi.
type StudentAttendanceStatusCounts struct {
	Hadir      int64
	Izin       int64
	Sakit      int64
	Alpha      int64
	Dispensasi int64
}

// StudentRecapRow adalah baris hasil agregasi rekap per siswa dari database (F5.5).
type StudentRecapRow struct {
	StudentID                  int64
	UserID                     int64
	StudentName                string
	NISN                       string
	ClassID                    int64
	ClassName                  string
	AcademicYearID             int64
	HadirSessions              int64
	IzinSessions               int64
	SakitSessions              int64
	AlphaSessions              int64
	DispensasiSessions         int64
	AlphaDaysInRange           int64
	TotalAlphaDaysAcademicYear int64
	LatestWarningThreshold     *int16
}

// LeaveRequestListParams adalah parameter query tervalidasi untuk daftar izin/sakit (F5.6-F5.9).
type LeaveRequestListParams struct {
	Page               int
	Limit              int
	DateFrom           time.Time
	DateTo             time.Time
	ClassID            *int64
	StudentID          *int64
	Type               string
	Status             string
	RestrictToClassIDs []int64
	RestrictToStudent  *int64
	DenyAll            bool
}

// PermissionLetterListParams adalah parameter query tervalidasi untuk daftar surat izin (F5.8).
type PermissionLetterListParams struct {
	Page               int
	Limit              int
	DateFrom           time.Time
	DateTo             time.Time
	ClassID            *int64
	StudentID          *int64
	Type               string
	RestrictToClassIDs []int64
	RestrictToStudent  *int64
	DenyAll            bool
}

// AlphaWarningListParams adalah parameter query untuk daftar log peringatan dini Alpha (F5.10).
type AlphaWarningListParams struct {
	Page               int
	Limit              int
	AcademicYearID     *int64
	ClassID            *int64
	StudentID          *int64
	Threshold          *int16
	RestrictToClassIDs []int64
	RestrictToStudent  *int64
	DenyAll            bool
}

// StudentAttendanceRepository mengelola query untuk EPIC 5:
// student_attendances, leave_requests, permission_letters, alpha_daily_summaries, dan alpha_warning_logs.
type StudentAttendanceRepository struct {
	db *gorm.DB
}

// NewStudentAttendanceRepository membuat instance StudentAttendanceRepository.
func NewStudentAttendanceRepository(db *gorm.DB) *StudentAttendanceRepository {
	return &StudentAttendanceRepository{db: db}
}

// WithTx mengembalikan salinan repository yang terikat ke transaksi tx.
func (r *StudentAttendanceRepository) WithTx(tx *gorm.DB) *StudentAttendanceRepository {
	return &StudentAttendanceRepository{db: tx}
}

func (r *StudentAttendanceRepository) baseAttendanceQuery() *gorm.DB {
	return r.db.
		Preload("Schedule.Class").
		Preload("Schedule.Subject").
		Preload("Schedule.Teacher.User").
		Preload("Student.User").
		Preload("Class")
}

// FindAttendanceByID mengambil satu baris absensi siswa berdasarkan ID.
func (r *StudentAttendanceRepository) FindAttendanceByID(id int64) (*models.StudentAttendance, error) {
	var rec models.StudentAttendance
	err := r.baseAttendanceQuery().First(&rec, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// FindAttendanceByIDForUpdate mengunci baris absensi siswa dengan SELECT ... FOR UPDATE.
func (r *StudentAttendanceRepository) FindAttendanceByIDForUpdate(id int64) (*models.StudentAttendance, error) {
	var rec models.StudentAttendance
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rec, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// FindAttendancesByScheduleID mengambil seluruh baris absensi siswa untuk satu schedule_id.
func (r *StudentAttendanceRepository) FindAttendancesByScheduleID(scheduleID int64) ([]models.StudentAttendance, error) {
	var items []models.StudentAttendance
	err := r.baseAttendanceQuery().
		Where("schedule_id = ?", scheduleID).
		Order("student_id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// HasManualChecklistSubmission memeriksa apakah jadwal sudah pernah disubmit
// via ceklis_guru atau fallback_wali_kelas (F5.1 / F5.2).
func (r *StudentAttendanceRepository) HasManualChecklistSubmission(scheduleID int64) (bool, error) {
	var count int64
	err := r.db.Model(&models.StudentAttendance{}).
		Where("schedule_id = ? AND source IN ?", scheduleID, []string{
			models.StudentAttendanceSourceCeklisGuru,
			models.StudentAttendanceSourceFallbackWaliKelas,
		}).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpsertStudentAttendance menyimpan atau memperbarui absensi siswa untuk pasangan (schedule_id, student_id).
// Penting: pada cabang DO UPDATE, kolom immutable (schedule_id, student_id, class_id, academic_year_id)
// TIDAK disentuh agar mematuhi trigger fn_student_attendances_guard di 020_student_attendances.sql.
func (r *StudentAttendanceRepository) UpsertStudentAttendance(rec *models.StudentAttendance) error {
	return r.db.Exec(`
		INSERT INTO student_attendances (schedule_id, student_id, status, source, recorded_by, note, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (schedule_id, student_id)
		DO UPDATE SET
			status = EXCLUDED.status,
			source = EXCLUDED.source,
			recorded_by = EXCLUDED.recorded_by,
			note = EXCLUDED.note,
			updated_at = EXCLUDED.updated_at
	`, rec.ScheduleID, rec.StudentID, rec.Status, rec.Source, rec.RecordedBy, rec.Note, rec.UpdatedAt).Error
}

// UpdateAttendance memperbarui status/note absensi siswa dengan optimistic locking pada updated_at (F5.3).
func (r *StudentAttendanceRepository) UpdateAttendance(id int64, expectedUpdatedAt time.Time, updates map[string]interface{}) (int64, error) {
	res := r.db.Model(&models.StudentAttendance{}).
		Where("id = ? AND updated_at = ?", id, expectedUpdatedAt).
		Updates(updates)
	return res.RowsAffected, res.Error
}

func (r *StudentAttendanceRepository) applyAttendanceFilters(db *gorm.DB, params StudentAttendanceListParams) *gorm.DB {
	query := db.Model(&models.StudentAttendance{}).
		Joins("JOIN schedules ON schedules.id = student_attendances.schedule_id")

	if params.DenyAll {
		return query.Where("1 = 0")
	}

	if !params.DateFrom.IsZero() {
		query = query.Where("schedules.schedule_date >= ?", params.DateFrom.Format("2006-01-02"))
	}
	if !params.DateTo.IsZero() {
		query = query.Where("schedules.schedule_date <= ?", params.DateTo.Format("2006-01-02"))
	}
	if params.RestrictToStudent != nil {
		query = query.Where("student_attendances.student_id = ?", *params.RestrictToStudent)
	} else if params.StudentID != nil {
		query = query.Where("student_attendances.student_id = ?", *params.StudentID)
	}

	if len(params.RestrictToClassIDs) > 0 && params.RestrictToTeacher != nil {
		tid := *params.RestrictToTeacher
		query = query.Where(`(
			student_attendances.class_id IN ?
			OR schedules.teacher_id = ?
			OR EXISTS (
				SELECT 1 FROM teacher_attendances ta
				WHERE ta.schedule_id = schedules.id AND ta.confirmed_teacher_id = ?
			)
		)`, params.RestrictToClassIDs, tid, tid)
	} else if len(params.RestrictToClassIDs) > 0 {
		query = query.Where("student_attendances.class_id IN ?", params.RestrictToClassIDs)
	} else if params.RestrictToTeacher != nil {
		tid := *params.RestrictToTeacher
		query = query.Where(`(
			schedules.teacher_id = ?
			OR EXISTS (
				SELECT 1 FROM teacher_attendances ta
				WHERE ta.schedule_id = schedules.id AND ta.confirmed_teacher_id = ?
			)
		)`, tid, tid)
	}

	if params.ClassID != nil {
		query = query.Where("student_attendances.class_id = ?", *params.ClassID)
	}
	if params.ScheduleID != nil {
		query = query.Where("student_attendances.schedule_id = ?", *params.ScheduleID)
	}
	if params.SubjectID != nil {
		query = query.Where("schedules.subject_id = ?", *params.SubjectID)
	}
	if params.AcademicYearID != nil {
		query = query.Where("student_attendances.academic_year_id = ?", *params.AcademicYearID)
	}
	if st := strings.TrimSpace(params.Status); st != "" {
		query = query.Where("student_attendances.status = ?", st)
	}
	if src := strings.TrimSpace(params.Source); src != "" {
		query = query.Where("student_attendances.source = ?", src)
	}

	return query
}

// ListAttendances mengambil daftar absensi siswa berhalaman beserta agregasi statusnya (F5.4).
func (r *StudentAttendanceRepository) ListAttendances(params StudentAttendanceListParams) ([]models.StudentAttendance, int64, StudentAttendanceStatusCounts, error) {
	var total int64
	if err := r.applyAttendanceFilters(r.db, params).Count(&total).Error; err != nil {
		return nil, 0, StudentAttendanceStatusCounts{}, err
	}

	type statusRow struct {
		Status string
		Count  int64
	}
	var rows []statusRow
	if err := r.applyAttendanceFilters(r.db, params).
		Select("student_attendances.status AS status, COUNT(*) AS count").
		Group("student_attendances.status").
		Scan(&rows).Error; err != nil {
		return nil, 0, StudentAttendanceStatusCounts{}, err
	}

	var counts StudentAttendanceStatusCounts
	for _, row := range rows {
		switch row.Status {
		case models.StudentAttendanceStatusHadir:
			counts.Hadir = row.Count
		case models.StudentAttendanceStatusIzin:
			counts.Izin = row.Count
		case models.StudentAttendanceStatusSakit:
			counts.Sakit = row.Count
		case models.StudentAttendanceStatusAlpha:
			counts.Alpha = row.Count
		case models.StudentAttendanceStatusDispensasi:
			counts.Dispensasi = row.Count
		}
	}

	var items []models.StudentAttendance
	offset := (params.Page - 1) * params.Limit
	err := r.applyAttendanceFilters(r.baseAttendanceQuery().Select("student_attendances.*"), params).
		Order("schedules.schedule_date DESC, schedules.start_time ASC, student_attendances.id DESC").
		Offset(offset).
		Limit(params.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, StudentAttendanceStatusCounts{}, err
	}

	return items, total, counts, nil
}

// ListAllAttendancesForExport mengambil seluruh baris detail absensi untuk export Excel (F5.5).
func (r *StudentAttendanceRepository) ListAllAttendancesForExport(params StudentAttendanceListParams) ([]models.StudentAttendance, error) {
	var items []models.StudentAttendance
	err := r.applyAttendanceFilters(r.baseAttendanceQuery().Select("student_attendances.*"), params).
		Order("schedules.schedule_date ASC, schedules.start_time ASC, student_attendances.student_id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// RecapByStudents menghitung rekap absensi per siswa dalam rentang tanggal beserta
// akumulasi hari Alpha harian (>75% jam pelajaran) dan ambang peringatan dini (F5.5 & F5.10).
func (r *StudentAttendanceRepository) RecapByStudents(params StudentAttendanceListParams) ([]StudentRecapRow, int64, StudentAttendanceStatusCounts, error) {
	if params.DenyAll {
		return []StudentRecapRow{}, 0, StudentAttendanceStatusCounts{}, nil
	}

	baseStudentQuery := r.db.Table("students").
		Joins("JOIN users ON users.id = students.user_id").
		Joins("JOIN classes ON classes.id = students.class_id").
		Where("users.archived_at IS NULL")

	if params.RestrictToStudent != nil {
		baseStudentQuery = baseStudentQuery.Where("students.id = ?", *params.RestrictToStudent)
	} else if params.StudentID != nil {
		baseStudentQuery = baseStudentQuery.Where("students.id = ?", *params.StudentID)
	}
	if len(params.RestrictToClassIDs) > 0 {
		baseStudentQuery = baseStudentQuery.Where("students.class_id IN ?", params.RestrictToClassIDs)
	}
	if params.ClassID != nil {
		baseStudentQuery = baseStudentQuery.Where("students.class_id = ?", *params.ClassID)
	}
	if params.AcademicYearID != nil {
		baseStudentQuery = baseStudentQuery.Where("classes.academic_year_id = ?", *params.AcademicYearID)
	}

	var total int64
	if err := baseStudentQuery.Count(&total).Error; err != nil {
		return nil, 0, StudentAttendanceStatusCounts{}, err
	}

	dateFromStr := params.DateFrom.Format("2006-01-02")
	dateToStr := params.DateTo.Format("2006-01-02")

	var rows []StudentRecapRow
	selectSQL := `
		students.id AS student_id,
		students.user_id AS user_id,
		users.name AS student_name,
		students.nisn AS nisn,
		classes.id AS class_id,
		classes.name AS class_name,
		classes.academic_year_id AS academic_year_id,
		COALESCE(SUM(CASE WHEN sa.status = 'hadir' THEN 1 ELSE 0 END), 0) AS hadir_sessions,
		COALESCE(SUM(CASE WHEN sa.status = 'izin' THEN 1 ELSE 0 END), 0) AS izin_sessions,
		COALESCE(SUM(CASE WHEN sa.status = 'sakit' THEN 1 ELSE 0 END), 0) AS sakit_sessions,
		COALESCE(SUM(CASE WHEN sa.status = 'alpha' THEN 1 ELSE 0 END), 0) AS alpha_sessions,
		COALESCE(SUM(CASE WHEN sa.status = 'dispensasi' THEN 1 ELSE 0 END), 0) AS dispensasi_sessions,
		(
			SELECT COUNT(*)
			FROM alpha_daily_summaries ads
			WHERE ads.student_id = students.id
			  AND ads.is_alpha = TRUE
			  AND ads.date >= ?::date
			  AND ads.date <= ?::date
		) AS alpha_days_in_range,
		(
			SELECT COUNT(*)
			FROM alpha_daily_summaries ads2
			WHERE ads2.student_id = students.id
			  AND ads2.academic_year_id = classes.academic_year_id
			  AND ads2.is_alpha = TRUE
		) AS total_alpha_days_academic_year,
		(
			SELECT MAX(awl.threshold)
			FROM alpha_warning_logs awl
			WHERE awl.student_id = students.id
			  AND awl.academic_year_id = classes.academic_year_id
		) AS latest_warning_threshold
	`

	q := baseStudentQuery.
		Select(selectSQL, dateFromStr, dateToStr).
		Joins(`LEFT JOIN schedules sch ON sch.class_id = students.class_id AND sch.schedule_date >= ?::date AND sch.schedule_date <= ?::date`, dateFromStr, dateToStr).
		Joins(`LEFT JOIN student_attendances sa ON sa.schedule_id = sch.id AND sa.student_id = students.id`).
		Group("students.id, students.user_id, users.name, students.nisn, classes.id, classes.name, classes.academic_year_id").
		Order("classes.name ASC, users.name ASC, students.id ASC")

	if params.Limit > 0 {
		offset := (params.Page - 1) * params.Limit
		q = q.Offset(offset).Limit(params.Limit)
	}

	if err := q.Scan(&rows).Error; err != nil {
		return nil, 0, StudentAttendanceStatusCounts{}, err
	}

	var totalCounts StudentAttendanceStatusCounts
	for _, rRow := range rows {
		totalCounts.Hadir += rRow.HadirSessions
		totalCounts.Izin += rRow.IzinSessions
		totalCounts.Sakit += rRow.SakitSessions
		totalCounts.Alpha += rRow.AlphaSessions
		totalCounts.Dispensasi += rRow.DispensasiSessions
	}

	return rows, total, totalCounts, nil
}

// =============================================================================
// Leave Requests (F5.6, F5.7, F5.9)
// =============================================================================

func (r *StudentAttendanceRepository) baseLeaveRequestQuery() *gorm.DB {
	return r.db.
		Preload("Student.User").
		Preload("Class")
}

// FindLeaveRequestByID mengambil satu pengajuan izin/sakit berdasarkan ID.
func (r *StudentAttendanceRepository) FindLeaveRequestByID(id int64) (*models.LeaveRequest, error) {
	var rec models.LeaveRequest
	err := r.baseLeaveRequestQuery().First(&rec, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// FindLeaveRequestByIDForUpdate mengunci baris leave_requests dengan SELECT ... FOR UPDATE.
func (r *StudentAttendanceRepository) FindLeaveRequestByIDForUpdate(id int64) (*models.LeaveRequest, error) {
	var rec models.LeaveRequest
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rec, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// FindLeaveRequestsByClassAndDate mengambil semua pengajuan izin/sakit pada kelas dan tanggal tertentu.
func (r *StudentAttendanceRepository) FindLeaveRequestsByClassAndDate(classID int64, date time.Time) ([]models.LeaveRequest, error) {
	var items []models.LeaveRequest
	err := r.baseLeaveRequestQuery().
		Where("class_id = ? AND date = ?::date", classID, date.Format("2006-01-02")).
		Order("created_at ASC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ExistsLeaveRequestForStudentDate memeriksa apakah siswa sudah punya pengajuan pada tanggal tersebut.
func (r *StudentAttendanceRepository) ExistsLeaveRequestForStudentDate(studentID int64, date time.Time) (bool, error) {
	var count int64
	err := r.db.Model(&models.LeaveRequest{}).
		Where("student_id = ? AND date = ?::date", studentID, date.Format("2006-01-02")).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateLeaveRequest menyimpan pengajuan izin/sakit baru (status = draft).
func (r *StudentAttendanceRepository) CreateLeaveRequest(rec *models.LeaveRequest) error {
	return r.db.Create(rec).Error
}

// UpdateLeaveRequestStatus memperbarui status verifikasi pengajuan izin/sakit (disetujui/ditolak).
func (r *StudentAttendanceRepository) UpdateLeaveRequestStatus(id int64, status string, reviewerID int64, reviewedAt time.Time) error {
	return r.db.Model(&models.LeaveRequest{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      status,
			"reviewed_by": reviewerID,
			"reviewed_at": reviewedAt,
		}).Error
}

// FindLessonSchedulesByClassAndDate mengambil seluruh jadwal pelajaran kelas pada tanggal tertentu (F5.7 & F5.10).
func (r *StudentAttendanceRepository) FindLessonSchedulesByClassAndDate(classID int64, date time.Time) ([]models.Schedule, error) {
	var items []models.Schedule
	err := r.db.
		Where("class_id = ? AND schedule_date = ?::date AND schedule_type = ?",
			classID, date.Format("2006-01-02"), models.ScheduleTypeLesson).
		Order("start_time ASC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// ListLeaveRequests mengambil daftar pengajuan izin/sakit berhalaman sesuai filter dan scoping (F5.6, F5.7, F5.9).
func (r *StudentAttendanceRepository) ListLeaveRequests(params LeaveRequestListParams) ([]models.LeaveRequest, int64, error) {
	applyFilters := func(db *gorm.DB) *gorm.DB {
		query := db.Model(&models.LeaveRequest{})
		if params.DenyAll {
			return query.Where("1 = 0")
		}
		if !params.DateFrom.IsZero() {
			query = query.Where("leave_requests.date >= ?::date", params.DateFrom.Format("2006-01-02"))
		}
		if !params.DateTo.IsZero() {
			query = query.Where("leave_requests.date <= ?::date", params.DateTo.Format("2006-01-02"))
		}
		if params.RestrictToStudent != nil {
			query = query.Where("leave_requests.student_id = ?", *params.RestrictToStudent)
		} else if params.StudentID != nil {
			query = query.Where("leave_requests.student_id = ?", *params.StudentID)
		}
		if len(params.RestrictToClassIDs) > 0 {
			query = query.Where("leave_requests.class_id IN ?", params.RestrictToClassIDs)
		}
		if params.ClassID != nil {
			query = query.Where("leave_requests.class_id = ?", *params.ClassID)
		}
		if tp := strings.TrimSpace(params.Type); tp != "" {
			query = query.Where("leave_requests.type = ?", tp)
		}
		if st := strings.TrimSpace(params.Status); st != "" {
			query = query.Where("leave_requests.status = ?", st)
		}
		return query
	}

	var total int64
	if err := applyFilters(r.db).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.LeaveRequest
	offset := (params.Page - 1) * params.Limit
	err := applyFilters(r.baseLeaveRequestQuery()).
		Order("leave_requests.date DESC, leave_requests.created_at DESC, leave_requests.id DESC").
		Offset(offset).
		Limit(params.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// =============================================================================
// Permission Letters (F5.8)
// =============================================================================

func (r *StudentAttendanceRepository) basePermissionLetterQuery() *gorm.DB {
	return r.db.
		Preload("Student.User").
		Preload("Uploader")
}

// CreatePermissionLetter menyimpan surat izin pulang/dispensasi (append-only, F5.8).
func (r *StudentAttendanceRepository) CreatePermissionLetter(rec *models.PermissionLetter) error {
	return r.db.Create(rec).Error
}

// FindPermissionLetterByID mengambil satu surat izin berdasarkan ID.
func (r *StudentAttendanceRepository) FindPermissionLetterByID(id int64) (*models.PermissionLetter, error) {
	var rec models.PermissionLetter
	err := r.basePermissionLetterQuery().First(&rec, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// ListPermissionLetters mengambil daftar surat izin berhalaman sesuai filter dan scoping (F5.8).
func (r *StudentAttendanceRepository) ListPermissionLetters(params PermissionLetterListParams) ([]models.PermissionLetter, int64, error) {
	applyFilters := func(db *gorm.DB) *gorm.DB {
		query := db.Model(&models.PermissionLetter{}).
			Joins("JOIN students ON students.id = permission_letters.student_id")
		if params.DenyAll {
			return query.Where("1 = 0")
		}
		if !params.DateFrom.IsZero() {
			query = query.Where("permission_letters.created_at >= ?", params.DateFrom)
		}
		if !params.DateTo.IsZero() {
			query = query.Where("permission_letters.created_at < ?", params.DateTo.AddDate(0, 0, 1))
		}
		if params.RestrictToStudent != nil {
			query = query.Where("permission_letters.student_id = ?", *params.RestrictToStudent)
		} else if params.StudentID != nil {
			query = query.Where("permission_letters.student_id = ?", *params.StudentID)
		}
		if len(params.RestrictToClassIDs) > 0 {
			query = query.Where("students.class_id IN ?", params.RestrictToClassIDs)
		}
		if params.ClassID != nil {
			query = query.Where("students.class_id = ?", *params.ClassID)
		}
		if tp := strings.TrimSpace(params.Type); tp != "" {
			query = query.Where("permission_letters.type = ?", tp)
		}
		return query
	}

	var total int64
	if err := applyFilters(r.db).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.PermissionLetter
	offset := (params.Page - 1) * params.Limit
	err := applyFilters(r.basePermissionLetterQuery().Select("permission_letters.*")).
		Order("permission_letters.created_at DESC, permission_letters.id DESC").
		Offset(offset).
		Limit(params.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// =============================================================================
// Alpha Tracking & Early Warning (F5.10)
// =============================================================================

// CountClassLessonSchedulesOnDate menghitung total jam pelajaran terjadwal suatu kelas pada satu tanggal.
func (r *StudentAttendanceRepository) CountClassLessonSchedulesOnDate(classID int64, date time.Time) (int16, error) {
	var count int64
	err := r.db.Model(&models.Schedule{}).
		Where("class_id = ? AND schedule_date = ?::date AND schedule_type = ?",
			classID, date.Format("2006-01-02"), models.ScheduleTypeLesson).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int16(count), nil
}

// CountStudentAlphaLessonsOnDate menghitung berapa jam pelajaran siswa berstatus alpha murni pada tanggal tersebut.
func (r *StudentAttendanceRepository) CountStudentAlphaLessonsOnDate(studentID, classID int64, date time.Time) (int16, error) {
	var count int64
	err := r.db.Model(&models.StudentAttendance{}).
		Joins("JOIN schedules ON schedules.id = student_attendances.schedule_id").
		Where("student_attendances.student_id = ? AND schedules.class_id = ? AND schedules.schedule_date = ?::date AND schedules.schedule_type = ? AND student_attendances.status = ?",
			studentID, classID, date.Format("2006-01-02"), models.ScheduleTypeLesson, models.StudentAttendanceStatusAlpha).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int16(count), nil
}

// CountStudentAlphaDaysInAcademicYear menghitung jumlah hari dengan is_alpha = TRUE dalam 1 tahun ajaran.
func (r *StudentAttendanceRepository) CountStudentAlphaDaysInAcademicYear(studentID, academicYearID int64) (int16, error) {
	var count int64
	err := r.db.Model(&models.AlphaDailySummary{}).
		Where("student_id = ? AND academic_year_id = ? AND is_alpha = TRUE", studentID, academicYearID).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int16(count), nil
}

// UpsertAlphaDailySummary menyimpan atau memperbarui ringkasan Alpha harian siswa (F5.10).
func (r *StudentAttendanceRepository) UpsertAlphaDailySummary(summary *models.AlphaDailySummary) error {
	return r.db.Exec(`
		INSERT INTO alpha_daily_summaries (student_id, class_id, academic_year_id, date, lesson_total, lesson_alpha, is_alpha, updated_at)
		VALUES (?, ?, ?, ?::date, ?, ?, ?, ?)
		ON CONFLICT (student_id, date)
		DO UPDATE SET
			class_id = EXCLUDED.class_id,
			academic_year_id = EXCLUDED.academic_year_id,
			lesson_total = EXCLUDED.lesson_total,
			lesson_alpha = EXCLUDED.lesson_alpha,
			is_alpha = EXCLUDED.is_alpha,
			updated_at = EXCLUDED.updated_at
	`, summary.StudentID, summary.ClassID, summary.AcademicYearID, summary.Date.Format("2006-01-02"),
		summary.LessonTotal, summary.LessonAlpha, summary.IsAlpha, summary.UpdatedAt).Error
}

// CreateAlphaWarningLog mencatat log peringatan dini ketika hari Alpha melewati ambang 3, 5, atau 9 hari (F5.10).
func (r *StudentAttendanceRepository) CreateAlphaWarningLog(logRec *models.AlphaWarningLog) error {
	return r.db.Create(logRec).Error
}

// ListAlphaWarnings mengambil daftar log peringatan dini Alpha berhalaman (F5.10).
func (r *StudentAttendanceRepository) ListAlphaWarnings(params AlphaWarningListParams) ([]models.AlphaWarningLog, int64, error) {
	applyFilters := func(db *gorm.DB) *gorm.DB {
		query := db.Model(&models.AlphaWarningLog{}).
			Joins("JOIN students ON students.id = alpha_warning_logs.student_id")
		if params.DenyAll {
			return query.Where("1 = 0")
		}
		if params.RestrictToStudent != nil {
			query = query.Where("alpha_warning_logs.student_id = ?", *params.RestrictToStudent)
		} else if params.StudentID != nil {
			query = query.Where("alpha_warning_logs.student_id = ?", *params.StudentID)
		}
		if len(params.RestrictToClassIDs) > 0 {
			query = query.Where("students.class_id IN ?", params.RestrictToClassIDs)
		}
		if params.ClassID != nil {
			query = query.Where("students.class_id = ?", *params.ClassID)
		}
		if params.AcademicYearID != nil {
			query = query.Where("alpha_warning_logs.academic_year_id = ?", *params.AcademicYearID)
		}
		if params.Threshold != nil {
			query = query.Where("alpha_warning_logs.threshold = ?", *params.Threshold)
		}
		return query
	}

	var total int64
	if err := applyFilters(r.db).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.AlphaWarningLog
	offset := (params.Page - 1) * params.Limit
	err := applyFilters(r.db.Preload("Student.User").Select("alpha_warning_logs.*")).
		Order("alpha_warning_logs.triggered_at DESC, alpha_warning_logs.id DESC").
		Offset(offset).
		Limit(params.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
