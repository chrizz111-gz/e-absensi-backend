package repositories

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"e-absensi-backend/models"
)

// ScheduleListParams adalah parameter query daftar jadwal yang sudah divalidasi
// dan menerapkan server-side scoping sesuai role.
type ScheduleListParams struct {
	Page      int
	Limit     int
	DateFrom  time.Time
	DateTo    time.Time
	ClassID   *int64
	TeacherID *int64
	SubjectID *int64
	Type      string

	// Scoping server-side:
	// - Jika RestrictToTeacherID != nil dan RestrictToClassIDs kosong: hanya jadwal guru tersebut.
	// - Jika RestrictToClassIDs terisi dan RestrictToTeacherID == nil: hanya jadwal kelas tersebut.
	// - Jika keduanya diset (akun multi-role guru_mapel + wali_kelas): jadwal guru ATAU kelas wali-nya.
	// - Jika DenyAll == true: langsung kembalikan 0 baris.
	DenyAll             bool
	RestrictToTeacherID *int64
	RestrictToClassIDs  []int64
}

// ScheduleRepository menangani query tabel schedules (Epic 4).
type ScheduleRepository struct {
	db *gorm.DB
}

// NewScheduleRepository membuat ScheduleRepository baru.
func NewScheduleRepository(db *gorm.DB) *ScheduleRepository {
	return &ScheduleRepository{db: db}
}

// WithTx mengembalikan repository yang terikat pada transaksi tertentu.
func (r *ScheduleRepository) WithTx(tx *gorm.DB) *ScheduleRepository {
	return &ScheduleRepository{db: tx}
}

// FindByID mencari satu jadwal berdasarkan primary key beserta relasi lengkap.
func (r *ScheduleRepository) FindByID(id int64) (*models.Schedule, error) {
	var schedule models.Schedule
	err := r.db.
		Preload("Class").
		Preload("Subject").
		Preload("Teacher.User").
		First(&schedule, "id = ?", id).Error
	if err != nil {
		return nil, translate(err)
	}
	return &schedule, nil
}

// FindByIDForUpdate mengunci baris jadwal sampai transaksi selesai.
func (r *ScheduleRepository) FindByIDForUpdate(id int64) (*models.Schedule, error) {
	var schedule models.Schedule
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&schedule, "id = ?", id).Error
	if err != nil {
		return nil, translate(err)
	}
	return &schedule, nil
}

// ExistsClassSlot memeriksa apakah kombinasi (class_id, schedule_date, start_time)
// sudah ada di database (dipakai untuk menentukan skipped_rows saat import).
func (r *ScheduleRepository) ExistsClassSlot(classID int64, date time.Time, startTime string, excludeID int64) (bool, error) {
	var count int64
	query := r.db.Model(&models.Schedule{}).
		Where("class_id = ? AND schedule_date = ? AND start_time = ?::time", classID, date.Format("2006-01-02"), startTime)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

// HasSlotConflict memeriksa apakah ada jadwal kelas atau guru yang bertumpuk
// pada tanggal dan rentang jam tersebut.
func (r *ScheduleRepository) HasSlotConflict(classID, teacherID int64, date time.Time, startTime, endTime string, excludeID int64) (bool, error) {
	var count int64
	dateStr := date.Format("2006-01-02")
	query := r.db.Model(&models.Schedule{}).
		Where("schedule_date = ?", dateStr).
		Where("(class_id = ? OR teacher_id = ?)", classID, teacherID).
		Where("start_time < ?::time AND end_time > ?::time", endTime, startTime)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

// BatchInsertIgnoreConflict menyimpan baris-baris jadwal valid secara batch
// dengan ON CONFLICT (class_id, schedule_date, start_time) DO NOTHING.
func (r *ScheduleRepository) BatchInsertIgnoreConflict(rows []models.Schedule) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	res := r.db.Omit(clause.Associations).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "class_id"},
			{Name: "schedule_date"},
			{Name: "start_time"},
		},
		DoNothing: true,
	}).Create(&rows)
	return res.RowsAffected, res.Error
}

// List mengambil satu halaman jadwal sesuai filter dan scoping role.
func (r *ScheduleRepository) List(params ScheduleListParams) ([]models.Schedule, int64, error) {
	if params.DenyAll {
		return []models.Schedule{}, 0, nil
	}

	applyFilters := func(db *gorm.DB) *gorm.DB {
		query := db.Model(&models.Schedule{}).
			Where("schedules.schedule_date >= ? AND schedules.schedule_date <= ?",
				params.DateFrom.Format("2006-01-02"),
				params.DateTo.Format("2006-01-02"),
			)

		switch {
		case params.RestrictToTeacherID != nil && len(params.RestrictToClassIDs) > 0:
			query = query.Where("(schedules.teacher_id = ? OR schedules.class_id IN ?)", *params.RestrictToTeacherID, params.RestrictToClassIDs)
		case params.RestrictToTeacherID != nil:
			query = query.Where("schedules.teacher_id = ?", *params.RestrictToTeacherID)
		case len(params.RestrictToClassIDs) > 0:
			query = query.Where("schedules.class_id IN ?", params.RestrictToClassIDs)
		}

		if params.ClassID != nil {
			query = query.Where("schedules.class_id = ?", *params.ClassID)
		}
		if params.TeacherID != nil {
			query = query.Where("schedules.teacher_id = ?", *params.TeacherID)
		}
		if params.SubjectID != nil {
			query = query.Where("schedules.subject_id = ?", *params.SubjectID)
		}
		if params.Type != "" {
			query = query.Where("schedules.schedule_type = ?", params.Type)
		}
		return query
	}

	var total int64
	if err := applyFilters(r.db).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []models.Schedule
	offset := (params.Page - 1) * params.Limit
	err := applyFilters(r.db).
		Preload("Class").
		Preload("Subject").
		Preload("Teacher.User").
		Order("schedules.schedule_date ASC, schedules.start_time ASC, schedules.id ASC").
		Offset(offset).
		Limit(params.Limit).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Update menerapkan perubahan pada baris jadwal dengan optimistic locking
// pada updated_at.
func (r *ScheduleRepository) Update(id int64, expectedUpdatedAt time.Time, updates map[string]interface{}) (int64, error) {
	res := r.db.Model(&models.Schedule{}).
		Where("id = ? AND updated_at = ?", id, expectedUpdatedAt).
		Updates(updates)
	return res.RowsAffected, res.Error
}

// Delete menghapus satu baris jadwal secara permanen.
func (r *ScheduleRepository) Delete(id int64) (int64, error) {
	res := r.db.Delete(&models.Schedule{}, id)
	return res.RowsAffected, res.Error
}

// HasActivityAttendance memeriksa apakah jadwal kegiatan sudah memiliki data
// peserta/absensi kegiatan dari Epic 8 (jika tabelnya sudah ada di DB).
func (r *ScheduleRepository) HasActivityAttendance(scheduleID int64) (bool, error) {
	return r.hasRowsInOptionalTables(scheduleID, []string{
		"activity_attendances",
		"activity_participants",
		"attendance_sessions",
	})
}

// HasAnyDependentRecords memeriksa apakah jadwal sudah memiliki data terkait
// (absensi siswa, kehadiran guru, jurnal mengajar, atau absensi kegiatan)
// pada tabel-tabel Epic 5/6/7/8 bila tabel tersebut sudah tersedia di DB.
func (r *ScheduleRepository) HasAnyDependentRecords(scheduleID int64) (bool, error) {
	return r.hasRowsInOptionalTables(scheduleID, []string{
		"attendance_sessions",
		"student_attendances",
		"student_attendance",
		"teacher_attendances",
		"teacher_attendance",
		"teaching_journals",
		"activity_attendances",
		"activity_participants",
	})
}

func (r *ScheduleRepository) hasRowsInOptionalTables(scheduleID int64, tables []string) (bool, error) {
	for _, table := range tables {
		var reg *string
		if err := r.db.Raw("SELECT to_regclass(?)", "public."+table).Scan(&reg).Error; err != nil {
			return false, err
		}
		if reg == nil || *reg == "" {
			continue
		}
		var hasCol bool
		if err := r.db.Raw(`
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_schema = 'public' AND table_name = ? AND column_name = 'schedule_id'
			)
		`, table).Scan(&hasCol).Error; err != nil {
			return false, err
		}
		if !hasCol {
			continue
		}
		var exists bool
		if err := r.db.Raw("SELECT EXISTS (SELECT 1 FROM "+table+" WHERE schedule_id = ?)", scheduleID).Scan(&exists).Error; err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}
