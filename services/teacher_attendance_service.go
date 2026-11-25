package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"e-absensi-backend/dto"
	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

// teacherQRTTL adalah masa berlaku QR Code dan kode cadangan (30 detik sesuai F6.1).
const teacherQRTTL = 30 * time.Second

// backupCodeAlphabet menghindari karakter ambigu (0/O, 1/I/L) agar mudah diketik guru.
const backupCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

type cachedTeacherQR struct {
	ScheduleID        int64
	ClassID           int64
	QRToken           string
	BackupCode        string
	GeneratedByUserID int64
	GeneratedByRole   string
	ExpiresAt         time.Time
}

type teacherQRStore struct {
	mu           sync.Mutex
	byScheduleID map[int64]cachedTeacherQR
	byToken      map[string]int64
	byBackupCode map[string]int64
}

func newTeacherQRStore() *teacherQRStore {
	return &teacherQRStore{
		byScheduleID: make(map[int64]cachedTeacherQR),
		byToken:      make(map[string]int64),
		byBackupCode: make(map[string]int64),
	}
}

// issue menyimpan token & kode cadangan baru untuk satu schedule_id dan
// menghapus token sebelumnya pada schedule_id tersebut agar hanya ada
// 1 token/kode aktif per jadwal pada satu waktu (F6.1).
func (s *teacherQRStore) issue(entry cachedTeacherQR, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Bersihkan token lama pada schedule yang sama jika ada.
	if prev, ok := s.byScheduleID[entry.ScheduleID]; ok {
		delete(s.byToken, prev.QRToken)
		delete(s.byBackupCode, prev.BackupCode)
		delete(s.byScheduleID, entry.ScheduleID)
	}

	// Bersihkan entri yang sudah lewat lebih dari 10 menit agar memori tidak membengkak.
	// Kita tetap menyimpan entri yang baru kedaluwarsa (< 10 menit) supaya bisa
	// membedakan 400 QR_CODE_EXPIRED vs 400 QR_CODE_INVALID.
	cutoff := now.Add(-10 * time.Minute)
	for schedID, item := range s.byScheduleID {
		if item.ExpiresAt.Before(cutoff) {
			delete(s.byToken, item.QRToken)
			delete(s.byBackupCode, item.BackupCode)
			delete(s.byScheduleID, schedID)
		}
	}

	s.byScheduleID[entry.ScheduleID] = entry
	s.byToken[entry.QRToken] = entry.ScheduleID
	s.byBackupCode[entry.BackupCode] = entry.ScheduleID
}

// lookup mencari entri berdasarkan qr_token atau backup_code dan mengembalikan
// apakah token ditemukan serta apakah sudah kedaluwarsa.
func (s *teacherQRStore) lookup(qrToken, backupCode string, now time.Time) (cachedTeacherQR, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var (
		schedID int64
		found   bool
		method  string
	)
	if qrToken != "" {
		schedID, found = s.byToken[qrToken]
		method = models.TeacherConfirmationQRScan
	} else if backupCode != "" {
		schedID, found = s.byBackupCode[strings.ToUpper(backupCode)]
		method = models.TeacherConfirmationBackupCode
	}
	if !found {
		return cachedTeacherQR{}, "", utils.ErrQRCodeInvalid
	}

	entry, ok := s.byScheduleID[schedID]
	if !ok {
		return cachedTeacherQR{}, "", utils.ErrQRCodeInvalid
	}
	if now.After(entry.ExpiresAt) {
		return cachedTeacherQR{}, "", utils.ErrQRCodeExpired
	}
	return entry, method, nil
}

// consume menghapus token & kode cadangan setelah konfirmasi kehadiran berhasil.
func (s *teacherQRStore) consume(scheduleID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if prev, ok := s.byScheduleID[scheduleID]; ok {
		delete(s.byToken, prev.QRToken)
		delete(s.byBackupCode, prev.BackupCode)
		delete(s.byScheduleID, scheduleID)
	}
}

// TeacherAttendanceService mengelola proses bisnis EPIC 6 (Teacher Attendance & QR).
type TeacherAttendanceService struct {
	db          *gorm.DB
	attendances *repositories.TeacherAttendanceRepository
	schedules   *repositories.ScheduleRepository
	classes     *repositories.ClassRepository
	teachers    *repositories.TeacherRepository
	students    *repositories.StudentRepository
	users       *repositories.UserRepository
	audit       AuditLogger
	qrStore     *teacherQRStore
	now         func() time.Time
}

// NewTeacherAttendanceService membuat instance TeacherAttendanceService.
func NewTeacherAttendanceService(db *gorm.DB, audit AuditLogger) *TeacherAttendanceService {
	if audit == nil {
		audit = NewNoopAuditLogger()
	}
	return &TeacherAttendanceService{
		db:          db,
		attendances: repositories.NewTeacherAttendanceRepository(db),
		schedules:   repositories.NewScheduleRepository(db),
		classes:     repositories.NewClassRepository(db),
		teachers:    repositories.NewTeacherRepository(db),
		students:    repositories.NewStudentRepository(db),
		users:       repositories.NewUserRepository(db),
		audit:       audit,
		qrStore:     newTeacherQRStore(),
		now:         time.Now,
	}
}

// StartAutoAbsentWorker menjalankan pengecekan periodik di background untuk
// menandai jadwal pelajaran yang sudah lewat tanpa konfirmasi sebagai tidak_hadir (F6.3).
func (s *TeacherAttendanceService) StartAutoAbsentWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = s.SyncExpiredUnconfirmedSchedules(ctx)
			}
		}
	}()
}

// SyncExpiredUnconfirmedSchedules menandai seluruh jadwal pelajaran yang sudah
// melewati jam selesai tetapi belum dikonfirmasi menjadi tidak_hadir (F6.3).
func (s *TeacherAttendanceService) SyncExpiredUnconfirmedSchedules(ctx context.Context) error {
	now := s.now().In(jakartaLocation())
	todayStr := now.Format(dateFormatYYYYMMDD)
	nowTimeStr := now.Format("15:04:05")

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txAtt := s.attendances.WithTx(tx)
		inserted, err := txAtt.MarkAbsentForExpiredSchedules(todayStr, nowTimeStr)
		if err != nil {
			return err
		}
		for _, item := range inserted {
			snapshot, err := marshalAuditSnapshot(map[string]interface{}{
				"id":          item.ID,
				"schedule_id": item.ScheduleID,
				"status":      models.TeacherAttendanceStatusTidakHadir,
				"marked_at":   now,
			})
			if err != nil {
				return err
			}
			if err := s.audit.LogTx(ctx, tx, AuditEvent{
				ActorID:    0, // NULL = proses otomatis sistem
				TargetType: TargetTypeTeacherAttendance,
				TargetID:   item.ID,
				Action:     ActionTeacherAttendanceMarkedAbsent,
				Snapshot:   snapshot,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// GenerateQR mendeteksi jadwal pelajaran aktif untuk kelas Siswa Inti atau
// Wali Kelas dan menerbitkan QR token + 6 karakter kode cadangan dengan TTL 30 detik (F6.1).
func (s *TeacherAttendanceService) GenerateQR(ctx context.Context, actorID int64, actorRoles []string, req dto.GenerateTeacherQRRequest) (*dto.GenerateTeacherQRResponse, error) {
	now := s.now().In(jakartaLocation())
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakartaLocation())
	nowTimeStr := now.Format("15:04:05")
	nowHHMM := now.Format("15:04")

	// Tentukan cakupan kelas dan role pembuat QR (siswa_inti vs wali_kelas).
	classIDs, generatedByRole, err := s.resolveQRGeneratorScope(actorID, actorRoles, req.ClassID)
	if err != nil {
		return nil, err
	}

	var targetSchedule *models.Schedule
	if req.ScheduleID != nil && *req.ScheduleID > 0 {
		sched, err := s.schedules.FindByID(*req.ScheduleID)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return nil, utils.ErrScheduleNotFound
			}
			return nil, utils.ErrInternal
		}
		if !containsInt64(classIDs, sched.ClassID) {
			return nil, utils.ErrForbidden.WithMessage("Anda hanya dapat membuat QR untuk jadwal kelas Anda sendiri")
		}
		if sched.ScheduleType != models.ScheduleTypeLesson {
			return nil, utils.ErrScheduleTypeNotSupported
		}
		schedStart := normalizeTimeDB(sched.StartTime)
		schedEnd := normalizeTimeDB(sched.EndTime)
		if !sameDate(sched.ScheduleDate, todayDate) || nowHHMM < schedStart || nowHHMM > schedEnd {
			return nil, utils.ErrScheduleNotActive
		}
		targetSchedule = sched
	} else {
		activeSchedules, err := s.attendances.FindActiveLessonSchedulesForClasses(classIDs, todayDate, nowTimeStr)
		if err != nil {
			return nil, utils.ErrInternal
		}
		if len(activeSchedules) == 0 {
			return nil, utils.ErrScheduleNotActive
		}
		targetSchedule = &activeSchedules[0]
	}

	// Cek apakah kehadiran guru untuk jadwal ini sudah tercatat.
	existing, err := s.attendances.FindByScheduleID(targetSchedule.ID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, utils.ErrInternal
	}
	if existing != nil {
		return nil, utils.ErrTeacherAttendanceAlreadyRecorded
	}

	qrToken, err := generateRandomHexToken(16) // 32 karakter hex
	if err != nil {
		return nil, utils.ErrInternal
	}
	backupCode, err := generateBackupCode(6)
	if err != nil {
		return nil, utils.ErrInternal
	}

	expiresAt := now.Add(teacherQRTTL)
	entry := cachedTeacherQR{
		ScheduleID:        targetSchedule.ID,
		ClassID:           targetSchedule.ClassID,
		QRToken:           qrToken,
		BackupCode:        backupCode,
		GeneratedByUserID: actorID,
		GeneratedByRole:   generatedByRole,
		ExpiresAt:         expiresAt,
	}
	s.qrStore.issue(entry, now)

	// Catat event log TEACHER_ATTENDANCE_QR_GENERATED.
	_ = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"schedule_id":          targetSchedule.ID,
			"class_id":             targetSchedule.ClassID,
			"generated_by_user_id": actorID,
			"generated_by_role":    generatedByRole,
			"expires_at":           expiresAt,
		})
		if err != nil {
			return err
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeTeacherAttendance,
			TargetID:   targetSchedule.ID,
			Action:     ActionTeacherAttendanceQRGenerated,
			Snapshot:   snapshot,
		})
	})

	return &dto.GenerateTeacherQRResponse{
		ScheduleID:        targetSchedule.ID,
		QRToken:           qrToken,
		BackupCode:        backupCode,
		ExpiresAt:         expiresAt,
		ExpiresInSeconds:  int(teacherQRTTL.Seconds()),
		GeneratedByUserID: actorID,
		GeneratedByRole:   generatedByRole,
		Schedule:          toScheduleResponse(targetSchedule),
	}, nil
}

// Confirm memvalidasi scan QR atau input kode cadangan dari Guru Mapel dan
// mencatat kehadiran guru (hadir atau guru_pengganti) pada teacher_attendances (F6.2).
func (s *TeacherAttendanceService) Confirm(ctx context.Context, actorID int64, req dto.ConfirmTeacherAttendanceRequest) (*dto.TeacherAttendanceResponse, error) {
	qrToken := strings.TrimSpace(req.QRToken)
	backupCode := strings.ToUpper(strings.TrimSpace(req.BackupCode))
	if qrToken == "" && backupCode == "" {
		return nil, utils.NewValidationError([]utils.FieldError{
			{Field: "qr_token", Code: utils.FieldRequired, Message: "qr_token atau backup_code wajib diisi"},
		})
	}

	teacher, err := s.teachers.FindByUserID(actorID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrForbidden.WithMessage("Akun Anda belum memiliki profil guru aktif")
		}
		return nil, utils.ErrInternal
	}

	now := s.now().In(jakartaLocation())
	entry, method, err := s.qrStore.lookup(qrToken, backupCode, now)
	if err != nil {
		return nil, err
	}

	schedule, err := s.schedules.FindByID(entry.ScheduleID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrScheduleNotFound
		}
		return nil, utils.ErrInternal
	}
	if schedule.ScheduleType != models.ScheduleTypeLesson {
		return nil, utils.ErrScheduleTypeNotSupported
	}

	// Cek apakah kehadiran untuk jadwal ini sudah tercatat sebelumnya.
	existing, err := s.attendances.FindByScheduleID(schedule.ID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, utils.ErrInternal
	}
	if existing != nil {
		return nil, utils.ErrTeacherAttendanceAlreadyRecorded
	}

	status := models.TeacherAttendanceStatusHadir
	if teacher.ID != schedule.TeacherID {
		status = models.TeacherAttendanceStatusGuruPengganti
	}

	confirmedTeacherID := teacher.ID
	generatedByUserID := entry.GeneratedByUserID
	generatedByRole := entry.GeneratedByRole
	confirmedAt := now

	record := &models.TeacherAttendance{
		ScheduleID:         schedule.ID,
		Status:             status,
		ConfirmedTeacherID: &confirmedTeacherID,
		ConfirmationMethod: &method,
		ConfirmedAt:        &confirmedAt,
		GeneratedByUserID:  &generatedByUserID,
		GeneratedByRole:    &generatedByRole,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txAtt := s.attendances.WithTx(tx)
		if err := txAtt.Create(record); err != nil {
			return err
		}

		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"id":                   record.ID,
			"schedule_id":          record.ScheduleID,
			"status":               record.Status,
			"scheduled_teacher_id": schedule.TeacherID,
			"confirmed_teacher_id": confirmedTeacherID,
			"confirmation_method":  method,
			"confirmed_at":         confirmedAt,
			"generated_by_user_id": generatedByUserID,
			"generated_by_role":    generatedByRole,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeTeacherAttendance,
			TargetID:   record.ID,
			Action:     ActionTeacherAttendanceConfirmed,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if isUniqueViolation(err) {
			return nil, utils.ErrTeacherAttendanceAlreadyRecorded
		}
		return nil, utils.ErrInternal
	}

	s.qrStore.consume(schedule.ID)

	full, err := s.attendances.FindByID(record.ID)
	if err != nil {
		return nil, utils.ErrInternal
	}
	resp := toTeacherAttendanceResponse(full)
	return &resp, nil
}

// List mengambil rekap kehadiran guru dengan filter dan scoping role (F6.4).
func (s *TeacherAttendanceService) List(ctx context.Context, actorID int64, actorRoles []string, filter dto.TeacherAttendanceListFilter) (*dto.TeacherAttendanceListResponse, error) {
	// Sinkronkan jadwal yang sudah lewat jam selesai menjadi tidak_hadir terlebih dahulu (F6.3).
	_ = s.SyncExpiredUnconfirmedSchedules(ctx)

	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	dateFrom, dateTo, err := s.resolveTeacherAttendanceDateRange(filter.DateFrom, filter.DateTo, filter.Preset)
	if err != nil {
		return nil, err
	}

	status := strings.ToLower(strings.TrimSpace(filter.Status))
	if status != "" &&
		status != models.TeacherAttendanceStatusHadir &&
		status != models.TeacherAttendanceStatusGuruPengganti &&
		status != models.TeacherAttendanceStatusTidakHadir {
		return nil, validationError("status", utils.FieldInvalidStatus, "status harus hadir, guru_pengganti, atau tidak_hadir")
	}

	params := repositories.TeacherAttendanceListParams{
		Page:      page,
		Limit:     limit,
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		TeacherID: filter.TeacherID,
		ClassID:   filter.ClassID,
		Status:    status,
	}

	if err := s.applyTeacherAttendanceScoping(actorID, actorRoles, &params); err != nil {
		return nil, err
	}

	items, total, counts, err := s.attendances.List(params)
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.TeacherAttendanceResponse, 0, len(items))
	for i := range items {
		respItems = append(respItems, toTeacherAttendanceResponse(&items[i]))
	}

	totalSessions := counts.TotalHadir + counts.TotalGuruPengganti + counts.TotalTidakHadir
	var rate float64
	if totalSessions > 0 {
		rate = float64(counts.TotalHadir+counts.TotalGuruPengganti) * 100.0 / float64(totalSessions)
	}

	return &dto.TeacherAttendanceListResponse{
		Items: respItems,
		Summary: dto.TeacherAttendanceSummaryStats{
			TotalSessions:         totalSessions,
			TotalHadir:            counts.TotalHadir,
			TotalGuruPengganti:    counts.TotalGuruPengganti,
			TotalTidakHadir:       counts.TotalTidakHadir,
			AttendanceRatePercent: roundTwoDecimals(rate),
		},
		DateFrom: dateFrom.Format(dateFormatYYYYMMDD),
		DateTo:   dateTo.Format(dateFormatYYYYMMDD),
		Meta:     buildPaginationMeta(page, limit, total),
	}, nil
}

// Get mengambil detail satu data kehadiran guru (F6.4).
func (s *TeacherAttendanceService) Get(ctx context.Context, actorID int64, actorRoles []string, id int64) (*dto.TeacherAttendanceResponse, error) {
	_ = s.SyncExpiredUnconfirmedSchedules(ctx)

	rec, err := s.attendances.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrTeacherAttendanceNotFound
		}
		return nil, utils.ErrInternal
	}

	if !hasAnyRole(actorRoles, models.RoleAdmin, models.RoleWakaKurikulum, models.RoleWakaKesiswaan) {
		teacher, err := s.teachers.FindByUserID(actorID)
		if err != nil {
			return nil, utils.ErrForbidden
		}
		isConfirmed := rec.ConfirmedTeacherID != nil && *rec.ConfirmedTeacherID == teacher.ID
		isScheduled := rec.Schedule.TeacherID == teacher.ID
		if !isConfirmed && !isScheduled {
			return nil, utils.ErrForbidden
		}
	}

	resp := toTeacherAttendanceResponse(rec)
	return &resp, nil
}

// Annotate memperbarui kolom keterangan pada kehadiran guru dengan optimistic
// locking pada updated_at (F6.4).
func (s *TeacherAttendanceService) Annotate(ctx context.Context, actorID, id int64, req dto.AnnotateTeacherAttendanceRequest) (*dto.TeacherAttendanceResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	var ketPtr *string
	if req.Keterangan != nil {
		trimmed := strings.TrimSpace(*req.Keterangan)
		if len([]rune(trimmed)) > 500 {
			return nil, validationError("keterangan", utils.FieldTooLong, "keterangan maksimal 500 karakter")
		}
		if trimmed != "" {
			ketPtr = &trimmed
		}
	}

	now := s.now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txAtt := s.attendances.WithTx(tx)

		locked, err := txAtt.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrTeacherAttendanceNotFound
			}
			return utils.ErrInternal
		}

		if !sameTimestamp(*req.UpdatedAt, locked.UpdatedAt) {
			return utils.ErrStaleData
		}

		beforeKet := locked.Keterangan
		updates := map[string]interface{}{
			"keterangan":            ketPtr,
			"keterangan_updated_by": actorID,
			"updated_at":            now,
		}

		rows, err := txAtt.Update(id, locked.UpdatedAt, updates)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}

		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"id":                id,
			"schedule_id":       locked.ScheduleID,
			"keterangan_before": beforeKet,
			"keterangan_after":  ketPtr,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeTeacherAttendance,
			TargetID:   id,
			Action:     ActionTeacherAttendanceAnnotated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, actorID, []string{models.RoleAdmin}, id)
}

// CorrectStatus mengoreksi status kehadiran guru oleh Admin / Waka Kurikulum
// dengan alasan wajib dan optimistic locking updated_at (F6.4 & migrasi 025).
func (s *TeacherAttendanceService) CorrectStatus(ctx context.Context, actorID, id int64, req dto.CorrectTeacherAttendanceRequest) (*dto.TeacherAttendanceResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	targetStatus := strings.ToLower(strings.TrimSpace(req.Status))
	if targetStatus != models.TeacherAttendanceStatusHadir &&
		targetStatus != models.TeacherAttendanceStatusGuruPengganti &&
		targetStatus != models.TeacherAttendanceStatusTidakHadir {
		return nil, validationError("status", utils.FieldInvalidStatus, "status harus hadir, guru_pengganti, atau tidak_hadir")
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" && req.Keterangan != nil {
		reason = strings.TrimSpace(*req.Keterangan)
	}
	if reason == "" {
		return nil, utils.ErrReasonRequired.WithField("reason", utils.CodeReasonRequired, "Alasan koreksi wajib diisi")
	}
	if len([]rune(reason)) > 500 {
		return nil, validationError("reason", utils.FieldTooLong, "Alasan koreksi maksimal 500 karakter")
	}

	now := s.now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txAtt := s.attendances.WithTx(tx)
		txSchedules := s.schedules.WithTx(tx)
		txTeachers := s.teachers.WithTx(tx)

		locked, err := txAtt.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrTeacherAttendanceNotFound
			}
			return utils.ErrInternal
		}

		if !sameTimestamp(*req.UpdatedAt, locked.UpdatedAt) {
			return utils.ErrStaleData
		}

		schedule, err := txSchedules.FindByID(locked.ScheduleID)
		if err != nil {
			return utils.ErrInternal
		}

		updates := map[string]interface{}{
			"keterangan":            reason,
			"keterangan_updated_by": actorID,
			"updated_at":            now,
		}

		switch targetStatus {
		case models.TeacherAttendanceStatusTidakHadir:
			updates["status"] = models.TeacherAttendanceStatusTidakHadir
			updates["confirmed_teacher_id"] = nil
			updates["confirmation_method"] = nil
			updates["confirmed_at"] = nil
			updates["generated_by_user_id"] = nil
			updates["generated_by_role"] = nil

		case models.TeacherAttendanceStatusHadir:
			confirmedID := schedule.TeacherID
			if req.ConfirmedTeacherID != nil && *req.ConfirmedTeacherID != schedule.TeacherID {
				return validationError("confirmed_teacher_id", utils.FieldInvalidID, "Untuk status hadir, confirmed_teacher_id harus sesuai dengan guru terjadwal")
			}
			updates["status"] = models.TeacherAttendanceStatusHadir
			updates["confirmed_teacher_id"] = confirmedID
			updates["confirmation_method"] = models.TeacherConfirmationKoreksiAdmin
			updates["confirmed_at"] = now
			updates["generated_by_user_id"] = nil
			updates["generated_by_role"] = nil

		case models.TeacherAttendanceStatusGuruPengganti:
			if req.ConfirmedTeacherID == nil || *req.ConfirmedTeacherID <= 0 {
				return validationError("confirmed_teacher_id", utils.FieldRequired, "confirmed_teacher_id wajib diisi untuk status guru_pengganti")
			}
			if *req.ConfirmedTeacherID == schedule.TeacherID {
				return validationError("confirmed_teacher_id", utils.FieldInvalidID, "Guru pengganti harus berbeda dari guru terjadwal")
			}
			subTeacher, err := txTeachers.FindByID(*req.ConfirmedTeacherID)
			if err != nil {
				if errors.Is(err, repositories.ErrNotFound) {
					return utils.ErrTeacherNotFound
				}
				return utils.ErrInternal
			}
			if !subTeacher.User.IsActive {
				return validationError("confirmed_teacher_id", utils.FieldInvalidStatus, "Akun guru pengganti sedang tidak aktif")
			}
			updates["status"] = models.TeacherAttendanceStatusGuruPengganti
			updates["confirmed_teacher_id"] = subTeacher.ID
			updates["confirmation_method"] = models.TeacherConfirmationKoreksiAdmin
			updates["confirmed_at"] = now
			updates["generated_by_user_id"] = nil
			updates["generated_by_role"] = nil
		}

		rows, err := txAtt.Update(id, locked.UpdatedAt, updates)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}

		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"id":                          id,
			"schedule_id":                 locked.ScheduleID,
			"status_before":               locked.Status,
			"status_after":                updates["status"],
			"confirmed_teacher_id_before": locked.ConfirmedTeacherID,
			"confirmed_teacher_id_after":  updates["confirmed_teacher_id"],
			"reason":                      reason,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeTeacherAttendance,
			TargetID:   id,
			Action:     ActionTeacherAttendanceCorrected,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, actorID, []string{models.RoleAdmin}, id)
}

// Export menghasilkan file laporan kehadiran guru dalam format Excel (.xlsx) (F6.5).
func (s *TeacherAttendanceService) Export(ctx context.Context, actorID int64, actorRoles []string, filter dto.TeacherAttendanceListFilter) ([]byte, string, error) {
	_ = s.SyncExpiredUnconfirmedSchedules(ctx)

	dateFrom, dateTo, err := s.resolveTeacherAttendanceDateRange(filter.DateFrom, filter.DateTo, filter.Preset)
	if err != nil {
		return nil, "", err
	}

	status := strings.ToLower(strings.TrimSpace(filter.Status))
	if status != "" &&
		status != models.TeacherAttendanceStatusHadir &&
		status != models.TeacherAttendanceStatusGuruPengganti &&
		status != models.TeacherAttendanceStatusTidakHadir {
		return nil, "", validationError("status", utils.FieldInvalidStatus, "status harus hadir, guru_pengganti, atau tidak_hadir")
	}

	params := repositories.TeacherAttendanceListParams{
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		TeacherID: filter.TeacherID,
		ClassID:   filter.ClassID,
		Status:    status,
	}
	if err := s.applyTeacherAttendanceScoping(actorID, actorRoles, &params); err != nil {
		return nil, "", err
	}

	items, err := s.attendances.ListAllForExport(params)
	if err != nil {
		return nil, "", utils.ErrInternal
	}

	rows := make([][]string, 0, len(items)+1)
	rows = append(rows, []string{
		"No",
		"Tanggal",
		"Jam Mulai",
		"Jam Selesai",
		"Kelas",
		"Mata Pelajaran",
		"Guru Terjadwal",
		"NIP Guru Terjadwal",
		"Status Kehadiran",
		"Guru Hadir / Pengganti",
		"Metode Konfirmasi",
		"Waktu Konfirmasi",
		"Keterangan",
	})

	for idx, item := range items {
		sched := item.Schedule
		className := ""
		if sched.Class.ID > 0 {
			className = sched.Class.Name
		}
		subjectName := ""
		if sched.Subject.ID > 0 {
			subjectName = fmt.Sprintf("%s - %s", sched.Subject.Code, sched.Subject.Name)
		}
		scheduledTeacherName := ""
		scheduledTeacherNIP := ""
		if sched.Teacher.ID > 0 {
			scheduledTeacherName = sched.Teacher.User.Name
			scheduledTeacherNIP = sched.Teacher.NIP
		}
		confirmedTeacherName := "-"
		if item.ConfirmedTeacher != nil && item.ConfirmedTeacher.ID > 0 {
			confirmedTeacherName = item.ConfirmedTeacher.User.Name
		}
		methodStr := "-"
		if item.ConfirmationMethod != nil {
			methodStr = *item.ConfirmationMethod
		}
		confirmedAtStr := "-"
		if item.ConfirmedAt != nil {
			confirmedAtStr = item.ConfirmedAt.In(jakartaLocation()).Format("2006-01-02 15:04:05")
		}
		ketStr := ""
		if item.Keterangan != nil {
			ketStr = *item.Keterangan
		}

		rows = append(rows, []string{
			fmt.Sprintf("%d", idx+1),
			sched.ScheduleDate.Format(dateFormatYYYYMMDD),
			normalizeTimeDB(sched.StartTime),
			normalizeTimeDB(sched.EndTime),
			className,
			subjectName,
			scheduledTeacherName,
			scheduledTeacherNIP,
			item.Status,
			confirmedTeacherName,
			methodStr,
			confirmedAtStr,
			ketStr,
		})
	}

	content, err := utils.BuildXLSX("Kehadiran Guru", rows)
	if err != nil {
		return nil, "", utils.ErrInternal
	}

	filename := fmt.Sprintf("rekap_kehadiran_guru_%s_sd_%s.xlsx",
		dateFrom.Format(dateFormatYYYYMMDD),
		dateTo.Format(dateFormatYYYYMMDD),
	)
	return content, filename, nil
}

func (s *TeacherAttendanceService) resolveQRGeneratorScope(actorID int64, actorRoles []string, requestedClassID *int64) ([]int64, string, error) {
	isSiswaInti := hasAnyRole(actorRoles, models.RoleSiswaInti)
	isWaliKelas := hasAnyRole(actorRoles, models.RoleWaliKelas)

	if isSiswaInti {
		student, err := s.students.FindByUserID(actorID)
		if err != nil || student.ClassID == nil {
			return nil, "", utils.ErrForbidden.WithMessage("Akun Siswa Inti belum ditempatkan pada kelas aktif")
		}
		if requestedClassID != nil && *requestedClassID != *student.ClassID {
			return nil, "", utils.ErrForbidden.WithMessage("Anda hanya dapat membuat QR untuk kelas Anda sendiri")
		}
		return []int64{*student.ClassID}, models.TeacherQRGeneratedBySiswaInti, nil
	}

	if isWaliKelas {
		waliClassIDs, err := s.classes.FindIDsByWaliKelas(actorID)
		if err != nil {
			return nil, "", utils.ErrInternal
		}
		if len(waliClassIDs) == 0 {
			return nil, "", utils.ErrForbidden.WithMessage("Akun Wali Kelas belum terhubung ke kelas aktif")
		}
		if requestedClassID != nil {
			if !containsInt64(waliClassIDs, *requestedClassID) {
				return nil, "", utils.ErrForbidden.WithMessage("Anda hanya dapat membuat QR untuk kelas perwalian Anda")
			}
			return []int64{*requestedClassID}, models.TeacherQRGeneratedByWaliKelas, nil
		}
		return waliClassIDs, models.TeacherQRGeneratedByWaliKelas, nil
	}

	return nil, "", utils.ErrForbidden
}

func (s *TeacherAttendanceService) applyTeacherAttendanceScoping(actorID int64, actorRoles []string, params *repositories.TeacherAttendanceListParams) error {
	if hasAnyRole(actorRoles, models.RoleAdmin, models.RoleWakaKurikulum, models.RoleWakaKesiswaan) {
		return nil
	}
	if hasAnyRole(actorRoles, models.RoleGuruMapel) {
		teacher, err := s.teachers.FindByUserID(actorID)
		if err != nil {
			params.DenyAll = true
			return nil
		}
		tid := teacher.ID
		params.RestrictToTeacherID = &tid
		return nil
	}
	return utils.ErrForbidden
}

func (s *TeacherAttendanceService) resolveTeacherAttendanceDateRange(fromStr, toStr, preset string) (time.Time, time.Time, error) {
	now := s.now().In(jakartaLocation())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakartaLocation())

	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "this_week":
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := today.AddDate(0, 0, -(weekday - 1))
		sunday := monday.AddDate(0, 0, 6)
		return monday, sunday, nil
	case "this_month":
		firstOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, jakartaLocation())
		lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
		return firstOfMonth, lastOfMonth, nil
	}

	fromStr = strings.TrimSpace(fromStr)
	toStr = strings.TrimSpace(toStr)

	var (
		fromDate time.Time
		toDate   time.Time
		err      error
	)

	if fromStr == "" && toStr == "" {
		// Default bulan berjalan untuk rekap kehadiran guru.
		firstOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, jakartaLocation())
		lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
		return firstOfMonth, lastOfMonth, nil
	}

	if fromStr != "" {
		fromDate, err = parseDateField("date_from", fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if toStr != "" {
		toDate, err = parseDateField("date_to", toStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	if fromStr != "" && toStr == "" {
		toDate = fromDate.AddDate(0, 1, -1)
	} else if fromStr == "" && toStr != "" {
		fromDate = toDate.AddDate(0, 0, -30)
	}

	if fromDate.After(toDate) {
		return time.Time{}, time.Time{}, validationError("date_to", utils.FieldInvalidDateRange, "date_from tidak boleh lebih besar dari date_to")
	}
	return fromDate, toDate, nil
}

func toTeacherAttendanceResponse(rec *models.TeacherAttendance) dto.TeacherAttendanceResponse {
	unlocked := rec.Status == models.TeacherAttendanceStatusHadir || rec.Status == models.TeacherAttendanceStatusGuruPengganti
	isSubstitute := rec.Status == models.TeacherAttendanceStatusGuruPengganti

	resp := dto.TeacherAttendanceResponse{
		ID:                     rec.ID,
		ScheduleID:             rec.ScheduleID,
		Status:                 rec.Status,
		ConfirmedTeacherID:     rec.ConfirmedTeacherID,
		IsSubstitute:           isSubstitute,
		ConfirmationMethod:     rec.ConfirmationMethod,
		ConfirmedAt:            rec.ConfirmedAt,
		GeneratedByUserID:      rec.GeneratedByUserID,
		GeneratedByRole:        rec.GeneratedByRole,
		Keterangan:             rec.Keterangan,
		KeteranganUpdatedBy:    rec.KeteranganUpdatedBy,
		AttendanceFormUnlocked: unlocked,
		CreatedAt:              rec.CreatedAt,
		UpdatedAt:              rec.UpdatedAt,
	}

	if rec.Schedule.ID > 0 {
		schedResp := toScheduleResponse(&rec.Schedule)
		resp.Schedule = &schedResp
		resp.ScheduledTeacher = schedResp.Teacher
	}
	if rec.ConfirmedTeacher != nil && rec.ConfirmedTeacher.ID > 0 {
		resp.ConfirmedTeacher = &dto.ScheduleTeacherSummary{
			ID:       rec.ConfirmedTeacher.ID,
			UserID:   rec.ConfirmedTeacher.UserID,
			Name:     rec.ConfirmedTeacher.User.Name,
			Email:    rec.ConfirmedTeacher.User.Email,
			NIP:      rec.ConfirmedTeacher.NIP,
			IsActive: rec.ConfirmedTeacher.User.IsActive,
		}
	}
	if rec.GeneratedByUser != nil && rec.GeneratedByUser.ID > 0 {
		role := ""
		if rec.GeneratedByRole != nil {
			role = *rec.GeneratedByRole
		}
		resp.GeneratedBy = &dto.TeacherAttendanceUserSummary{
			ID:   rec.GeneratedByUser.ID,
			Name: rec.GeneratedByUser.Name,
			Role: role,
		}
	}
	return resp
}

func generateRandomHexToken(numBytes int) (string, error) {
	b := make([]byte, numBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func generateBackupCode(length int) (string, error) {
	out := make([]byte, length)
	maxIdx := big.NewInt(int64(len(backupCodeAlphabet)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, maxIdx)
		if err != nil {
			return "", err
		}
		out[i] = backupCodeAlphabet[n.Int64()]
	}
	return string(out), nil
}

func hasAnyRole(actorRoles []string, targetRoles ...string) bool {
	for _, ar := range actorRoles {
		for _, tr := range targetRoles {
			if ar == tr {
				return true
			}
		}
	}
	return false
}

func roundTwoDecimals(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100.0
}

var (
	jakartaOnce sync.Once
	jakartaLoc  *time.Location
)

// jakartaLocation mengembalikan zona waktu Asia/Jakarta (WIB, UTC+7).
// Jika tzdata tidak tersedia di OS host, menggunakan FixedZone WIB (+07:00).
func jakartaLocation() *time.Location {
	jakartaOnce.Do(func() {
		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil || loc == nil {
			loc = time.FixedZone("WIB", 7*3600)
		}
		jakartaLoc = loc
	})
	return jakartaLoc
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return errors.Is(err, gorm.ErrDuplicatedKey)
}

