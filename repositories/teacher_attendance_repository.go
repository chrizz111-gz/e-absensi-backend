package repositories

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/models"
)

// TeacherAttendanceListParams adalah parameter query tervalidasi untuk daftar
// dan export kehadiran guru (F6.4 & F6.5).
type TeacherAttendanceListParams struct {
	Page                int
	Limit               int
	DateFrom            time.Time
	DateTo              time.Time
	TeacherID           *int64
	ClassID             *int64
	Status              string
	RestrictToTeacherID *int64
	DenyAll             bool
}

// TeacherAttendanceSummaryCounts menyimpan hasil agregasi status kehadiran guru.
type TeacherAttendanceSummaryCounts struct {
	TotalHadir         int64
	TotalGuruPengganti int64
	TotalTidakHadir    int64
}

// AbsentMarkedItem mewakili satu baris jadwal yang otomatis ditandai tidak_hadir (F6.3).
type AbsentMarkedItem struct {
	ID         int64
	ScheduleID int64
}

// TeacherAttendanceRepository mengelola query tabel teacher_attendances (Epic 6).
type TeacherAttendanceRepository struct {
	db *gorm.DB
}

// NewTeacherAttendanceRepository membuat instance TeacherAttendanceRepository.
func NewTeacherAttendanceRepository(db *gorm.DB) *TeacherAttendanceRepository {
	return &TeacherAttendanceRepository{db: db}
}

// WithTx mengembalikan salinan repository yang terikat ke transaksi tx.
func (r *TeacherAttendanceRepository) WithTx(tx *gorm.DB) *TeacherAttendanceRepository {
	return &TeacherAttendanceRepository{db: tx}
}

func (r *TeacherAttendanceRepository) baseQuery() *gorm.DB {
	return r.db.
		Preload("Schedule.Class").
		Preload("Schedule.Subject").
		Preload("Schedule.Teacher.User").
		Preload("ConfirmedTeacher.User").
		Preload("GeneratedByUser").
		Preload("KeteranganEditor")
}

// FindByID mengambil satu baris kehadiran guru berdasarkan ID.
func (r *TeacherAttendanceRepository) FindByID(id int64) (*models.TeacherAttendance, error) {
	var rec models.TeacherAttendance
	err := r.baseQuery().First(&rec, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// FindByIDForUpdate mengunci baris kehadiran guru dengan SELECT ... FOR UPDATE.
func (r *TeacherAttendanceRepository) FindByIDForUpdate(id int64) (*models.TeacherAttendance, error) {
	var rec models.TeacherAttendance
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&rec, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// FindByScheduleID mengambil kehadiran guru berdasarkan schedule_id.
func (r *TeacherAttendanceRepository) FindByScheduleID(scheduleID int64) (*models.TeacherAttendance, error) {
	var rec models.TeacherAttendance
	err := r.baseQuery().Where("schedule_id = ?", scheduleID).First(&rec).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// FindActiveLessonSchedulesForClasses mencari jadwal pelajaran yang sedang
// berlangsung saat ini (schedule_date = date AND start_time <= nowTime <= end_time)
// untuk daftar class_id tertentu (F6.1).
func (r *TeacherAttendanceRepository) FindActiveLessonSchedulesForClasses(classIDs []int64, date time.Time, nowTime string) ([]models.Schedule, error) {
	if len(classIDs) == 0 {
		return nil, nil
	}
	var items []models.Schedule
	err := r.db.
		Preload("Class").
		Preload("Subject").
		Preload("Teacher.User").
		Where("class_id IN ? AND schedule_date = ? AND schedule_type = ? AND start_time <= ?::time AND end_time >= ?::time",
			classIDs, date.Format("2006-01-02"), models.ScheduleTypeLesson, nowTime, nowTime).
		Order("start_time ASC, id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// Create menyimpan satu baris kehadiran guru baru.
func (r *TeacherAttendanceRepository) Create(rec *models.TeacherAttendance) error {
	return r.db.Create(rec).Error
}

// Update memperbarui baris kehadiran guru dengan optimistic locking pada updated_at.
func (r *TeacherAttendanceRepository) Update(id int64, expectedUpdatedAt time.Time, updates map[string]interface{}) (int64, error) {
	res := r.db.Model(&models.TeacherAttendance{}).
		Where("id = ? AND updated_at = ?", id, expectedUpdatedAt).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// MarkAbsentForExpiredSchedules memasukkan baris status = 'tidak_hadir' secara
// otomatis untuk semua jadwal pelajaran yang sudah melewati jam selesai tetapi
// belum memiliki baris di teacher_attendances (F6.3).
func (r *TeacherAttendanceRepository) MarkAbsentForExpiredSchedules(todayDate, nowTime string) ([]AbsentMarkedItem, error) {
	var inserted []AbsentMarkedItem
	err := r.db.Raw(`
		INSERT INTO teacher_attendances (schedule_id, status)
		SELECT s.id, ?
		FROM schedules s
		LEFT JOIN teacher_attendances ta ON ta.schedule_id = s.id
		WHERE ta.id IS NULL
		  AND s.schedule_type = ?
		  AND (
		        s.schedule_date < ?::date
		        OR (s.schedule_date = ?::date AND s.end_time < ?::time)
		      )
		ON CONFLICT (schedule_id) DO NOTHING
		RETURNING id, schedule_id
	`, models.TeacherAttendanceStatusTidakHadir, models.ScheduleTypeLesson, todayDate, todayDate, nowTime).Scan(&inserted).Error
	if err != nil {
		return nil, err
	}
	return inserted, nil
}

func (r *TeacherAttendanceRepository) applyListFilters(db *gorm.DB, params TeacherAttendanceListParams) *gorm.DB {
	query := db.Model(&models.TeacherAttendance{}).
		Joins("JOIN schedules ON schedules.id = teacher_attendances.schedule_id")

	if params.DenyAll {
		return query.Where("1 = 0")
	}

	if !params.DateFrom.IsZero() {
		query = query.Where("schedules.schedule_date >= ?", params.DateFrom.Format("2006-01-02"))
	}
	if !params.DateTo.IsZero() {
		query = query.Where("schedules.schedule_date <= ?", params.DateTo.Format("2006-01-02"))
	}
	if params.RestrictToTeacherID != nil {
		tid := *params.RestrictToTeacherID
		query = query.Where("(schedules.teacher_id = ? OR teacher_attendances.confirmed_teacher_id = ?)", tid, tid)
	} else if params.TeacherID != nil {
		tid := *params.TeacherID
		query = query.Where("(schedules.teacher_id = ? OR teacher_attendances.confirmed_teacher_id = ?)", tid, tid)
	}
	if params.ClassID != nil {
		query = query.Where("schedules.class_id = ?", *params.ClassID)
	}
	if status := strings.TrimSpace(params.Status); status != "" {
		query = query.Where("teacher_attendances.status = ?", status)
	}
	return query
}

// List mengambil daftar kehadiran guru berhalaman beserta ringkasan statistiknya (F6.4).
func (r *TeacherAttendanceRepository) List(params TeacherAttendanceListParams) ([]models.TeacherAttendance, int64, TeacherAttendanceSummaryCounts, error) {
	var total int64
	if err := r.applyListFilters(r.db, params).Count(&total).Error; err != nil {
		return nil, 0, TeacherAttendanceSummaryCounts{}, err
	}

	type statusRow struct {
		Status string
		Count  int64
	}
	var rows []statusRow
	if err := r.applyListFilters(r.db, params).
		Select("teacher_attendances.status AS status, COUNT(*) AS count").
		Group("teacher_attendances.status").
		Scan(&rows).Error; err != nil {
		return nil, 0, TeacherAttendanceSummaryCounts{}, err
	}

	var summary TeacherAttendanceSummaryCounts
	for _, row := range rows {
		switch row.Status {
		case models.TeacherAttendanceStatusHadir:
			summary.TotalHadir = row.Count
		case models.TeacherAttendanceStatusGuruPengganti:
			summary.TotalGuruPengganti = row.Count
		case models.TeacherAttendanceStatusTidakHadir:
			summary.TotalTidakHadir = row.Count
		}
	}

	var items []models.TeacherAttendance
	offset := (params.Page - 1) * params.Limit
	err := r.applyListFilters(r.baseQuery().Select("teacher_attendances.*"), params).
		Order("schedules.schedule_date DESC, schedules.start_time ASC, teacher_attendances.id DESC").
		Offset(offset).
		Limit(params.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, TeacherAttendanceSummaryCounts{}, err
	}

	return items, total, summary, nil
}

// ListAllForExport mengambil seluruh baris kehadiran guru sesuai filter untuk diexport ke Excel (F6.5).
func (r *TeacherAttendanceRepository) ListAllForExport(params TeacherAttendanceListParams) ([]models.TeacherAttendance, error) {
	var items []models.TeacherAttendance
	err := r.applyListFilters(r.baseQuery().Select("teacher_attendances.*"), params).
		Order("schedules.schedule_date ASC, schedules.start_time ASC, teacher_attendances.id ASC").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}
