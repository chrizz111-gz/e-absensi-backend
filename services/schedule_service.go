package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"e-absensi-backend/dto"
	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

const (
	timeFormatHHMM       = "15:04"
	timeFormatHHMMSS     = "15:04:05"
	importPreviewHoldTTL = 30 * time.Minute
)

var scheduleTemplateHeaders = []string{
	"Tanggal (YYYY-MM-DD)",
	"Jam Mulai (HH:MM)",
	"Jam Selesai (HH:MM)",
	"Nama Kelas",
	"Kode Mata Pelajaran",
	"Email Guru",
}

// ScheduleUsageChecker adalah interface pengecekan keterpakai jadwal oleh
// modul absensi/jurnal/kegiatan (Epic 5, 6, 7, 8).
type ScheduleUsageChecker interface {
	HasActivityAttendance(scheduleID int64) (bool, error)
	HasAnyDependentRecords(scheduleID int64) (bool, error)
}

type cachedImportBatch struct {
	ActorID      int64
	ExpiresAt    time.Time
	ValidRows    []models.Schedule
	SkippedCount int64
}

type scheduleImportStore struct {
	mu      sync.Mutex
	batches map[string]cachedImportBatch
}

func newScheduleImportStore() *scheduleImportStore {
	return &scheduleImportStore{
		batches: make(map[string]cachedImportBatch),
	}
}

func (s *scheduleImportStore) put(token string, batch cachedImportBatch, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.batches {
		if !v.ExpiresAt.After(now) {
			delete(s.batches, k)
		}
	}
	s.batches[token] = batch
}

func (s *scheduleImportStore) take(token string, actorID int64, now time.Time) (cachedImportBatch, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	batch, ok := s.batches[token]
	if !ok {
		return cachedImportBatch{}, false
	}
	if !batch.ExpiresAt.After(now) || batch.ActorID != actorID {
		if !batch.ExpiresAt.After(now) {
			delete(s.batches, token)
		}
		return cachedImportBatch{}, false
	}
	delete(s.batches, token)
	return batch, true
}

// ScheduleService menangani logika bisnis Manajemen Jadwal (Epic 4 F4.1-F4.6).
type ScheduleService struct {
	db        *gorm.DB
	schedules *repositories.ScheduleRepository
	classes   *repositories.ClassRepository
	subjects  *repositories.SubjectRepository
	teachers  *repositories.TeacherRepository
	students  *repositories.StudentRepository
	users     *repositories.UserRepository
	usage     ScheduleUsageChecker
	imports   *scheduleImportStore
	audit     AuditLogger
	now       func() time.Time
}

// NewScheduleService membuat ScheduleService baru.
func NewScheduleService(db *gorm.DB, audit AuditLogger) *ScheduleService {
	if audit == nil {
		audit = NewNoopAuditLogger()
	}
	schedRepo := repositories.NewScheduleRepository(db)
	return &ScheduleService{
		db:        db,
		schedules: schedRepo,
		classes:   repositories.NewClassRepository(db),
		subjects:  repositories.NewSubjectRepository(db),
		teachers:  repositories.NewTeacherRepository(db),
		students:  repositories.NewStudentRepository(db),
		users:     repositories.NewUserRepository(db),
		usage:     schedRepo,
		imports:   newScheduleImportStore(),
		audit:     audit,
		now:       time.Now,
	}
}

// GenerateTemplate menghasilkan file .xlsx template jadwal (F4.1) dengan header
// baku dan 1 baris contoh.
func (s *ScheduleService) GenerateTemplate(ctx context.Context) ([]byte, error) {
	rows := [][]string{
		scheduleTemplateHeaders,
		{"2026-11-18", "07:00", "08:30", "X RPL 1", "MTK-10", "guru@sekolah.sch.id"},
	}
	return utils.BuildXLSX("Template Jadwal", rows)
}

// PreviewImport membaca file .xlsx, memvalidasi header dan setiap baris tanpa
// menyimpan ke DB, memisahkan ke valid_rows, skipped_rows, dan error_rows, lalu
// menyimpan batch valid ke cache sementara dengan import_token (F4.2).
func (s *ScheduleService) PreviewImport(ctx context.Context, actorID int64, fileBytes []byte) (*dto.ScheduleImportPreviewResponse, error) {
	rows, err := utils.ReadXLSX(fileBytes)
	if err != nil || len(rows) == 0 {
		return nil, utils.ErrImportFileInvalid
	}

	if !isValidScheduleHeader(rows[0]) {
		return nil, utils.ErrImportFileInvalid
	}

	validPreviews := make([]dto.ScheduleImportRowPreview, 0)
	skippedPreviews := make([]dto.ScheduleImportRowPreview, 0)
	errorPreviews := make([]dto.ScheduleImportRowPreview, 0)
	modelsToInsert := make([]models.Schedule, 0)

	type slotWindow struct {
		date  string
		start string
		end   string
		row   int
	}
	seenClassSlots := make(map[int64][]slotWindow)
	seenTeacherSlots := make(map[int64][]slotWindow)

	for idx := 1; idx < len(rows); idx++ {
		rowNum := idx + 1
		cells := padCells(rows[idx], 6)
		if isBlankRow(cells) {
			continue
		}

		rawDate := strings.TrimSpace(cells[0])
		rawStart := strings.TrimSpace(cells[1])
		rawEnd := strings.TrimSpace(cells[2])
		rawClass := strings.TrimSpace(cells[3])
		rawSubject := strings.ToUpper(strings.TrimSpace(cells[4]))
		rawEmail := strings.ToLower(strings.TrimSpace(cells[5]))

		preview := dto.ScheduleImportRowPreview{
			RowNumber:    rowNum,
			Date:         rawDate,
			StartTime:    rawStart,
			EndTime:      rawEnd,
			ClassName:    rawClass,
			SubjectCode:  rawSubject,
			TeacherEmail: rawEmail,
		}

		var rowErrors []string

		parsedDate, err := time.Parse(dateFormatYYYYMMDD, rawDate)
		if err != nil {
			rowErrors = append(rowErrors, "Format Tanggal harus YYYY-MM-DD")
		}

		normStart, startOK := normalizeTimeHHMM(rawStart)
		if !startOK {
			rowErrors = append(rowErrors, "Format Jam Mulai harus HH:MM")
		} else {
			preview.StartTime = normStart
		}

		normEnd, endOK := normalizeTimeHHMM(rawEnd)
		if !endOK {
			rowErrors = append(rowErrors, "Format Jam Selesai harus HH:MM")
		} else {
			preview.EndTime = normEnd
		}

		if startOK && endOK && normStart >= normEnd {
			rowErrors = append(rowErrors, "Jam Mulai harus lebih awal dari Jam Selesai")
		}

		var classModel *models.Class
		if rawClass == "" {
			rowErrors = append(rowErrors, "Nama Kelas wajib diisi")
		} else {
			c, err := s.classes.FindActiveByName(rawClass)
			if err != nil {
				rowErrors = append(rowErrors, fmt.Sprintf("Kelas '%s' tidak ditemukan atau tidak aktif", rawClass))
			} else {
				classModel = c
				preview.ClassID = c.ID
			}
		}

		var subjectModel *models.Subject
		if rawSubject == "" {
			rowErrors = append(rowErrors, "Kode Mata Pelajaran wajib diisi")
		} else {
			subj, err := s.subjects.FindByCode(rawSubject)
			if err != nil || !subj.IsActive {
				rowErrors = append(rowErrors, fmt.Sprintf("Kode Mata Pelajaran '%s' tidak ditemukan atau tidak aktif", rawSubject))
			} else {
				subjectModel = subj
				preview.SubjectID = subj.ID
			}
		}

		var teacherModel *models.Teacher
		if rawEmail == "" {
			rowErrors = append(rowErrors, "Email Guru wajib diisi")
		} else {
			t, err := s.teachers.FindByUserEmail(rawEmail)
			if err != nil || !t.User.IsActive {
				rowErrors = append(rowErrors, fmt.Sprintf("Guru dengan email '%s' tidak ditemukan atau akun tidak aktif", rawEmail))
			} else {
				roles, rErr := s.users.GetRoles(t.UserID)
				if rErr != nil || !hasRole(roles, models.RoleGuruMapel) {
					rowErrors = append(rowErrors, fmt.Sprintf("Akun '%s' tidak memiliki role guru_mapel aktif", rawEmail))
				} else {
					teacherModel = t
					preview.TeacherID = t.ID
				}
			}
		}

		if len(rowErrors) > 0 {
			preview.Errors = rowErrors
			errorPreviews = append(errorPreviews, preview)
			continue
		}

		// Cek apakah kombinasi (class_id, date, start_time) sudah ada di DB -> skipped_rows.
		existsSlot, err := s.schedules.ExistsClassSlot(classModel.ID, parsedDate, normStart, 0)
		if err != nil {
			return nil, utils.ErrInternal
		}
		if existsSlot {
			preview.Reason = "Jadwal kelas pada tanggal dan jam mulai ini sudah ada di database (dilewati)"
			skippedPreviews = append(skippedPreviews, preview)
			continue
		}

		// Cek bentrok dengan jadwal lain di DB (overlap rentang jam kelas atau guru).
		dbConflict, err := s.schedules.HasSlotConflict(classModel.ID, teacherModel.ID, parsedDate, normStart, normEnd, 0)
		if err != nil {
			return nil, utils.ErrInternal
		}
		if dbConflict {
			preview.Errors = []string{"Slot waktu bertumpuk dengan jadwal kelas atau guru yang sudah ada di database"}
			errorPreviews = append(errorPreviews, preview)
			continue
		}

		// Cek bentrok di dalam file Excel yang sama.
		dateStr := parsedDate.Format(dateFormatYYYYMMDD)
		skipCurrentRow := false
		for _, prev := range seenClassSlots[classModel.ID] {
			if prev.date == dateStr && normStart < prev.end && normEnd > prev.start {
				if normStart == prev.start {
					preview.Reason = fmt.Sprintf("Duplikat slot kelas dengan baris %d di dalam file (dilewati)", prev.row)
					skippedPreviews = append(skippedPreviews, preview)
					skipCurrentRow = true
					break
				}
				rowErrors = append(rowErrors, fmt.Sprintf("Jam kelas bertumpuk dengan baris %d di dalam file", prev.row))
				break
			}
		}
		if skipCurrentRow {
			continue
		}
		for _, prev := range seenTeacherSlots[teacherModel.ID] {
			if prev.date == dateStr && normStart < prev.end && normEnd > prev.start {
				rowErrors = append(rowErrors, fmt.Sprintf("Guru mengajar di dua kelas pada waktu yang bertumpuk dengan baris %d", prev.row))
				break
			}
		}
		if len(rowErrors) > 0 {
			preview.Errors = rowErrors
			errorPreviews = append(errorPreviews, preview)
			continue
		}

		seenClassSlots[classModel.ID] = append(seenClassSlots[classModel.ID], slotWindow{
			date: dateStr, start: normStart, end: normEnd, row: rowNum,
		})
		seenTeacherSlots[teacherModel.ID] = append(seenTeacherSlots[teacherModel.ID], slotWindow{
			date: dateStr, start: normStart, end: normEnd, row: rowNum,
		})

		validPreviews = append(validPreviews, preview)
		actorCopy := actorID
		modelsToInsert = append(modelsToInsert, models.Schedule{
			ScheduleDate: parsedDate,
			StartTime:    normStart,
			EndTime:      normEnd,
			ClassID:      classModel.ID,
			SubjectID:    subjectModel.ID,
			TeacherID:    teacherModel.ID,
			ScheduleType: models.ScheduleTypeLesson,
			CreatedBy:    &actorCopy,
			Version:      1,
		})
	}

	now := s.now()
	expiresAt := now.Add(importPreviewHoldTTL)
	token := uuid.NewString()

	s.imports.put(token, cachedImportBatch{
		ActorID:      actorID,
		ExpiresAt:    expiresAt,
		ValidRows:    modelsToInsert,
		SkippedCount: int64(len(skippedPreviews)),
	}, now)

	totalRows := len(validPreviews) + len(skippedPreviews) + len(errorPreviews)
	return &dto.ScheduleImportPreviewResponse{
		ImportToken:  token,
		ExpiresAt:    expiresAt,
		TotalRows:    totalRows,
		ValidCount:   len(validPreviews),
		SkippedCount: len(skippedPreviews),
		ErrorCount:   len(errorPreviews),
		ValidRows:    validPreviews,
		SkippedRows:  skippedPreviews,
		ErrorRows:    errorPreviews,
	}, nil
}

// ConfirmImport menyimpan hasil preview yang sudah divalidasi secara batch
// dalam 1 transaksi DB dengan ON CONFLICT DO NOTHING (F4.2).
func (s *ScheduleService) ConfirmImport(ctx context.Context, actorID int64, req dto.ScheduleImportConfirmRequest) (*dto.ScheduleImportConfirmResponse, error) {
	now := s.now()
	token := strings.TrimSpace(req.ImportToken)
	batch, ok := s.imports.take(token, actorID, now)
	if !ok {
		return nil, utils.ErrImportNotValidated
	}

	var importedCount int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txSchedules := s.schedules.WithTx(tx)
		inserted, err := txSchedules.BatchInsertIgnoreConflict(batch.ValidRows)
		if err != nil {
			return err
		}
		importedCount = inserted

		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"valid_rows":     len(batch.ValidRows),
			"imported_count": importedCount,
			"skipped_count":  batch.SkippedCount + (int64(len(batch.ValidRows)) - importedCount),
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeSchedule,
			TargetID:   0,
			Action:     ActionScheduleImported,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapScheduleWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	skippedTotal := batch.SkippedCount + (int64(len(batch.ValidRows)) - importedCount)
	return &dto.ScheduleImportConfirmResponse{
		ImportedCount: importedCount,
		SkippedCount:  skippedTotal,
	}, nil
}

// List mengambil daftar jadwal dengan filter rentang tanggal (default 1 minggu
// berjalan) dan menerapkan server-side scoping ketat berdasarkan role (F4.3).
func (s *ScheduleService) List(ctx context.Context, actorID int64, actorRoles []string, filter dto.ScheduleListFilter) (*dto.ScheduleListResponse, error) {
	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	dateFrom, dateTo, err := s.resolveDateRange(filter.DateFrom, filter.DateTo)
	if err != nil {
		return nil, err
	}

	schedType := strings.ToLower(strings.TrimSpace(filter.Type))
	if schedType != "" && schedType != models.ScheduleTypeLesson && schedType != models.ScheduleTypeActivity {
		return nil, validationError("type", utils.FieldInvalidScheduleType, "type harus pelajaran atau kegiatan")
	}

	params := repositories.ScheduleListParams{
		Page:      page,
		Limit:     limit,
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		ClassID:   filter.ClassID,
		TeacherID: filter.TeacherID,
		SubjectID: filter.SubjectID,
		Type:      schedType,
	}

	if err := s.applyRoleScoping(actorID, actorRoles, &params); err != nil {
		return nil, err
	}

	items, total, err := s.schedules.List(params)
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.ScheduleResponse, 0, len(items))
	for i := range items {
		respItems = append(respItems, toScheduleResponse(&items[i]))
	}

	return &dto.ScheduleListResponse{
		Items:    respItems,
		DateFrom: dateFrom.Format(dateFormatYYYYMMDD),
		DateTo:   dateTo.Format(dateFormatYYYYMMDD),
		Meta:     buildPaginationMeta(page, limit, total),
	}, nil
}

// Get mengambil detail satu jadwal dengan validasi cakupan akses sesuai role (F4.3).
func (s *ScheduleService) Get(ctx context.Context, actorID int64, actorRoles []string, id int64) (*dto.ScheduleResponse, error) {
	schedule, err := s.schedules.FindByID(id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrScheduleNotFound
		}
		return nil, utils.ErrInternal
	}

	if err := s.authorizeScheduleView(actorID, actorRoles, schedule); err != nil {
		return nil, err
	}

	resp := toScheduleResponse(schedule)
	return &resp, nil
}

// Update memperbarui baris jadwal (Admin & Waka Kurikulum) dengan optimistic
// locking (updated_at) dan pengecekan bentrok slot (F4.4).
func (s *ScheduleService) Update(ctx context.Context, actorID, id int64, req dto.UpdateScheduleRequest) (*dto.ScheduleResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	now := s.now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txSchedules := s.schedules.WithTx(tx)
		txClasses := s.classes.WithTx(tx)
		txSubjects := s.subjects.WithTx(tx)
		txTeachers := s.teachers.WithTx(tx)
		txUsers := s.users.WithTx(tx)

		locked, err := txSchedules.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrScheduleNotFound
			}
			return utils.ErrInternal
		}

		if !sameTimestamp(*req.UpdatedAt, locked.UpdatedAt) {
			return utils.ErrStaleData
		}

		currentFull, err := txSchedules.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		before := toScheduleResponse(currentFull)

		effectiveDate := locked.ScheduleDate
		effectiveStart := normalizeTimeDB(locked.StartTime)
		effectiveEnd := normalizeTimeDB(locked.EndTime)
		effectiveClassID := locked.ClassID
		effectiveSubjectID := locked.SubjectID
		effectiveTeacherID := locked.TeacherID

		updates := make(map[string]interface{})

		if req.Date != nil {
			parsedDate, err := parseDateField("date", *req.Date)
			if err != nil {
				return err
			}
			effectiveDate = parsedDate
			if !sameDate(parsedDate, locked.ScheduleDate) {
				updates["schedule_date"] = parsedDate
			}
		}

		if req.StartTime != nil {
			normStart, ok := normalizeTimeHHMM(*req.StartTime)
			if !ok {
				return validationError("start_time", utils.FieldInvalidTime, "start_time harus berformat HH:MM")
			}
			if normStart != effectiveStart {
				updates["start_time"] = normStart
			}
			effectiveStart = normStart
		}

		if req.EndTime != nil {
			normEnd, ok := normalizeTimeHHMM(*req.EndTime)
			if !ok {
				return validationError("end_time", utils.FieldInvalidTime, "end_time harus berformat HH:MM")
			}
			if normEnd != effectiveEnd {
				updates["end_time"] = normEnd
			}
			effectiveEnd = normEnd
		}

		if effectiveStart >= effectiveEnd {
			return validationError("end_time", utils.FieldInvalidTimeRange, "start_time harus lebih awal dari end_time")
		}

		if req.ClassID != nil {
			classObj, err := txClasses.FindByID(*req.ClassID)
			if err != nil {
				if errors.Is(err, repositories.ErrNotFound) {
					return utils.ErrClassNotFound
				}
				return utils.ErrInternal
			}
			if !classObj.IsActive {
				return validationError("class_id", utils.FieldClassInactive, "Kelas harus berstatus aktif")
			}
			if *req.ClassID != locked.ClassID {
				updates["class_id"] = *req.ClassID
			}
			effectiveClassID = *req.ClassID
		}

		if req.SubjectID != nil {
			subjObj, err := txSubjects.FindByID(*req.SubjectID)
			if err != nil {
				if errors.Is(err, repositories.ErrNotFound) {
					return utils.ErrSubjectNotFound
				}
				return utils.ErrInternal
			}
			if !subjObj.IsActive {
				return validationError("subject_id", utils.FieldSubjectInactive, "Mata pelajaran harus berstatus aktif")
			}
			if *req.SubjectID != locked.SubjectID {
				updates["subject_id"] = *req.SubjectID
			}
			effectiveSubjectID = *req.SubjectID
		}

		if req.TeacherID != nil {
			teacherObj, err := txTeachers.FindByID(*req.TeacherID)
			if err != nil {
				if errors.Is(err, repositories.ErrNotFound) {
					return utils.ErrTeacherNotFound
				}
				return utils.ErrInternal
			}
			if !teacherObj.User.IsActive {
				return validationError("teacher_id", utils.FieldInvalidStatus, "Akun guru sedang tidak aktif")
			}
			roles, err := txUsers.GetRoles(teacherObj.UserID)
			if err != nil {
				return utils.ErrInternal
			}
			if !hasRole(roles, models.RoleGuruMapel) {
				return utils.ErrAccountNotTeacherRole.WithField("teacher_id", utils.CodeAccountNotTeacherRole, "Akun guru harus memiliki role guru_mapel")
			}
			if *req.TeacherID != locked.TeacherID {
				updates["teacher_id"] = *req.TeacherID
			}
			effectiveTeacherID = *req.TeacherID
		}
		_ = effectiveSubjectID

		if len(updates) == 0 {
			return nil
		}

		conflict, err := txSchedules.HasSlotConflict(effectiveClassID, effectiveTeacherID, effectiveDate, effectiveStart, effectiveEnd, id)
		if err != nil {
			return utils.ErrInternal
		}
		if conflict {
			return utils.ErrScheduleSlotConflict
		}

		updates["updated_at"] = now
		rows, err := txSchedules.Update(id, locked.UpdatedAt, updates)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}

		updatedFull, err := txSchedules.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		after := toScheduleResponse(updatedFull)
		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"before": before,
			"after":  after,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeSchedule,
			TargetID:   id,
			Action:     ActionScheduleUpdated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if mapped := mapScheduleWriteError(err); mapped != nil {
			return nil, mapped
		}
		return nil, utils.ErrInternal
	}

	return s.Get(ctx, actorID, []string{models.RoleAdmin}, id)
}

// ConvertToActivity mengubah jadwal pelajaran menjadi kegiatan (F4.6).
// Wajib mengisi activity_name, mempertahankan teacher_id, dan menolak 409
// SCHEDULE_ALREADY_ACTIVITY bila sudah berstatus kegiatan.
func (s *ScheduleService) ConvertToActivity(ctx context.Context, actorID, id int64, req dto.ConvertScheduleToActivityRequest) (*dto.ScheduleResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	activityName := strings.TrimSpace(req.ActivityName)
	if activityName == "" {
		return nil, validationError("activity_name", utils.FieldRequired, "activity_name wajib diisi")
	}
	if len([]rune(activityName)) > 150 {
		return nil, validationError("activity_name", utils.FieldTooLong, "activity_name maksimal 150 karakter")
	}

	now := s.now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txSchedules := s.schedules.WithTx(tx)

		locked, err := txSchedules.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrScheduleNotFound
			}
			return utils.ErrInternal
		}

		if !sameTimestamp(*req.UpdatedAt, locked.UpdatedAt) {
			return utils.ErrStaleData
		}

		if locked.ScheduleType == models.ScheduleTypeActivity {
			return utils.ErrScheduleAlreadyActivity
		}

		currentFull, err := txSchedules.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		before := toScheduleResponse(currentFull)

		rows, err := txSchedules.Update(id, locked.UpdatedAt, map[string]interface{}{
			"schedule_type": models.ScheduleTypeActivity,
			"activity_name": activityName,
			"updated_at":    now,
		})
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}

		updatedFull, err := txSchedules.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		after := toScheduleResponse(updatedFull)
		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"before": before,
			"after":  after,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeSchedule,
			TargetID:   id,
			Action:     ActionScheduleConvertedToActivity,
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

// RevertToLesson mengembalikan jadwal kegiatan menjadi pelajaran (F4.6).
// Mengecek terlebih dahulu apakah sudah ada data peserta/absensi kegiatan
// (Epic 8); jika sudah ada, ditolak 409 SCHEDULE_IN_USE.
func (s *ScheduleService) RevertToLesson(ctx context.Context, actorID, id int64, req dto.RevertScheduleToLessonRequest) (*dto.ScheduleResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	now := s.now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txSchedules := s.schedules.WithTx(tx)

		locked, err := txSchedules.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrScheduleNotFound
			}
			return utils.ErrInternal
		}

		if !sameTimestamp(*req.UpdatedAt, locked.UpdatedAt) {
			return utils.ErrStaleData
		}

		if locked.ScheduleType != models.ScheduleTypeActivity {
			return utils.ErrScheduleAlreadyLesson
		}

		if s.usage != nil {
			used, err := s.usage.HasActivityAttendance(id)
			if err != nil {
				return utils.ErrInternal
			}
			if used {
				return utils.ErrScheduleInUse.WithMessage("Jadwal kegiatan tidak dapat dikembalikan ke pelajaran karena sudah memiliki data peserta/absensi kegiatan")
			}
		}

		updates := map[string]interface{}{
			"schedule_type": models.ScheduleTypeLesson,
			"activity_name": nil,
			"updated_at":    now,
		}
		if req.SubjectID != nil && *req.SubjectID > 0 {
			subj, err := s.subjects.WithTx(tx).FindByID(*req.SubjectID)
			if err != nil {
				if errors.Is(err, repositories.ErrNotFound) {
					return utils.ErrSubjectNotFound
				}
				return utils.ErrInternal
			}
			if !subj.IsActive {
				return validationError("subject_id", utils.FieldSubjectInactive, "Mata pelajaran harus berstatus aktif")
			}
			updates["subject_id"] = *req.SubjectID
		}

		currentFull, err := txSchedules.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		before := toScheduleResponse(currentFull)

		rows, err := txSchedules.Update(id, locked.UpdatedAt, updates)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}

		updatedFull, err := txSchedules.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		after := toScheduleResponse(updatedFull)
		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"before": before,
			"after":  after,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeSchedule,
			TargetID:   id,
			Action:     ActionScheduleRevertedToLesson,
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

// Delete menghapus jadwal secara permanen (khusus Admin, F4.5). Jika jadwal
// sudah memiliki data terkait (absensi/jurnal/kegiatan), ditolak 409 SCHEDULE_IN_USE.
func (s *ScheduleService) Delete(ctx context.Context, actorID, id int64) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txSchedules := s.schedules.WithTx(tx)

		locked, err := txSchedules.FindByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrScheduleNotFound
			}
			return utils.ErrInternal
		}
		_ = locked

		if s.usage != nil {
			inUse, err := s.usage.HasAnyDependentRecords(id)
			if err != nil {
				return utils.ErrInternal
			}
			if inUse {
				return utils.ErrScheduleInUse
			}
		}

		currentFull, err := txSchedules.FindByID(id)
		if err != nil {
			return utils.ErrInternal
		}
		snapshot, err := marshalAuditSnapshot(toScheduleResponse(currentFull))
		if err != nil {
			return utils.ErrInternal
		}
		if err := s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeSchedule,
			TargetID:   id,
			Action:     ActionScheduleDeleted,
			Snapshot:   snapshot,
		}); err != nil {
			return err
		}

		rows, err := txSchedules.Delete(id)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrScheduleNotFound
		}
		return nil
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return appErr
		}
		if isForeignKeyViolation(err) {
			return utils.ErrScheduleInUse
		}
		return utils.ErrInternal
	}
	return nil
}

// applyRoleScoping menerapkan pembatasan cakupan jadwal berdasarkan role user:
//   - Admin, Waka Kurikulum, Waka Kesiswaan: seluruh sekolah.
//   - Guru Mapel: paksa filter hanya teacher_id miliknya (abaikan manipulasi query param).
//   - Wali Kelas, Siswa Inti, Siswa: paksa filter hanya class_id kelasnya (tolak 403 bila meminta kelas lain).
func (s *ScheduleService) applyRoleScoping(actorID int64, actorRoles []string, params *repositories.ScheduleListParams) error {
	hasSchoolWide := false
	hasGuru := false
	hasWali := false
	hasStudent := false

	for _, r := range actorRoles {
		switch r {
		case models.RoleAdmin, models.RoleWakaKurikulum, models.RoleWakaKesiswaan:
			hasSchoolWide = true
		case models.RoleGuruMapel:
			hasGuru = true
		case models.RoleWaliKelas:
			hasWali = true
		case models.RoleSiswa, models.RoleSiswaInti:
			hasStudent = true
		}
	}

	if hasSchoolWide {
		return nil
	}

	if hasStudent {
		student, err := s.students.FindByUserID(actorID)
		if err != nil || student.ClassID == nil {
			if params.ClassID != nil {
				return utils.ErrForbidden
			}
			params.DenyAll = true
			return nil
		}
		if params.ClassID != nil && *params.ClassID != *student.ClassID {
			return utils.ErrForbidden
		}
		params.ClassID = student.ClassID
		params.RestrictToClassIDs = []int64{*student.ClassID}
		return nil
	}

	var myTeacherID *int64
	if hasGuru {
		teacher, err := s.teachers.FindByUserID(actorID)
		if err == nil {
			id := teacher.ID
			myTeacherID = &id
		}
	}

	var myWaliClassIDs []int64
	if hasWali {
		ids, err := s.classes.FindIDsByWaliKelas(actorID)
		if err != nil {
			return utils.ErrInternal
		}
		myWaliClassIDs = ids
	}

	switch {
	case hasGuru && !hasWali:
		// Guru Mapel murni: paksa teacher_id miliknya, abaikan manipulasi query param.
		if myTeacherID == nil {
			params.DenyAll = true
			return nil
		}
		params.TeacherID = myTeacherID
		params.RestrictToTeacherID = myTeacherID
		return nil

	case hasWali && !hasGuru:
		// Wali Kelas murni: hanya boleh akses kelasnya, tolak 403 bila meminta kelas lain.
		if params.ClassID != nil && !containsInt64(myWaliClassIDs, *params.ClassID) {
			return utils.ErrForbidden
		}
		if len(myWaliClassIDs) == 0 {
			params.DenyAll = true
			return nil
		}
		params.RestrictToClassIDs = myWaliClassIDs
		return nil

	case hasGuru && hasWali:
		// Multi-role Guru Mapel + Wali Kelas:
		// - Jika memfilter class_id yang bukan kelas walinya, hanya izinkan jika tidak melanggar (atau jika memaksa kelas lain tanpa kaitan, tolak 403 bila bukan jadwal mengajarnya).
		if params.ClassID != nil && !containsInt64(myWaliClassIDs, *params.ClassID) {
			if myTeacherID == nil {
				return utils.ErrForbidden
			}
			params.TeacherID = myTeacherID
			params.RestrictToTeacherID = myTeacherID
			return nil
		}
		if myTeacherID == nil && len(myWaliClassIDs) == 0 {
			params.DenyAll = true
			return nil
		}
		params.TeacherID = nil
		params.RestrictToTeacherID = myTeacherID
		params.RestrictToClassIDs = myWaliClassIDs
		return nil

	default:
		return utils.ErrForbidden
	}
}

// authorizeScheduleView memverifikasi apakah user boleh melihat detail satu jadwal.
func (s *ScheduleService) authorizeScheduleView(actorID int64, actorRoles []string, schedule *models.Schedule) error {
	for _, r := range actorRoles {
		switch r {
		case models.RoleAdmin, models.RoleWakaKurikulum, models.RoleWakaKesiswaan:
			return nil
		}
	}

	hasGuru := false
	hasWali := false
	hasStudent := false
	for _, r := range actorRoles {
		switch r {
		case models.RoleGuruMapel:
			hasGuru = true
		case models.RoleWaliKelas:
			hasWali = true
		case models.RoleSiswa, models.RoleSiswaInti:
			hasStudent = true
		}
	}

	if hasStudent {
		student, err := s.students.FindByUserID(actorID)
		if err != nil || student.ClassID == nil || *student.ClassID != schedule.ClassID {
			return utils.ErrForbidden
		}
		return nil
	}

	if hasGuru {
		teacher, err := s.teachers.FindByUserID(actorID)
		if err == nil && teacher.ID == schedule.TeacherID {
			return nil
		}
	}

	if hasWali {
		waliClassIDs, err := s.classes.FindIDsByWaliKelas(actorID)
		if err == nil && containsInt64(waliClassIDs, schedule.ClassID) {
			return nil
		}
	}

	return utils.ErrForbidden
}

// resolveDateRange memvalidasi date_from dan date_to, atau mengisi default
// 1 minggu berjalan (Senin s.d. Minggu) bila kosong.
func (s *ScheduleService) resolveDateRange(rawFrom, rawTo string) (time.Time, time.Time, error) {
	fromStr := strings.TrimSpace(rawFrom)
	toStr := strings.TrimSpace(rawTo)

	if fromStr == "" && toStr == "" {
		now := s.now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Minggu = hari ke-7
		}
		monday := time.Date(now.Year(), now.Month(), now.Day()-(weekday-1), 0, 0, 0, 0, time.UTC)
		sunday := monday.AddDate(0, 0, 6)
		return monday, sunday, nil
	}

	if fromStr == "" {
		toDate, err := parseDateField("date_to", toStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		return toDate.AddDate(0, 0, -6), toDate, nil
	}

	fromDate, err := parseDateField("date_from", fromStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	if toStr == "" {
		return fromDate, fromDate.AddDate(0, 0, 6), nil
	}

	toDate, err := parseDateField("date_to", toStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	if toDate.Before(fromDate) {
		return time.Time{}, time.Time{}, validationError("date_to", utils.FieldInvalidDateRange, "date_to tidak boleh sebelum date_from")
	}

	return fromDate, toDate, nil
}

func isValidScheduleHeader(header []string) bool {
	if len(header) < 6 {
		return false
	}
	expectedPrefixes := []string{
		"tanggal",
		"jam mulai",
		"jam selesai",
		"nama kelas",
		"kode mata pelajaran",
		"email guru",
	}
	for i, prefix := range expectedPrefixes {
		cell := strings.ToLower(strings.TrimSpace(header[i]))
		if !strings.HasPrefix(cell, prefix) {
			return false
		}
	}
	return true
}

func padCells(row []string, minLen int) []string {
	if len(row) >= minLen {
		return row
	}
	padded := make([]string, minLen)
	copy(padded, row)
	return padded
}

func isBlankRow(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func normalizeTimeHHMM(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	if t, err := time.Parse(timeFormatHHMM, trimmed); err == nil {
		return t.Format(timeFormatHHMM), true
	}
	if t, err := time.Parse(timeFormatHHMMSS, trimmed); err == nil {
		return t.Format(timeFormatHHMM), true
	}
	return "", false
}

func normalizeTimeDB(raw string) string {
	if norm, ok := normalizeTimeHHMM(raw); ok {
		return norm
	}
	return strings.TrimSpace(raw)
}

func toScheduleResponse(schedule *models.Schedule) dto.ScheduleResponse {
	var classSummary *dto.ScheduleClassSummary
	if schedule.Class != nil && schedule.Class.ID > 0 {
		classSummary = &dto.ScheduleClassSummary{
			ID:         schedule.Class.ID,
			Name:       schedule.Class.Name,
			GradeLevel: schedule.Class.GradeLevel,
			IsActive:   schedule.Class.IsActive,
		}
	}

	var subjectID *int64
	var subjectSummary *dto.ScheduleSubjectSummary
	if schedule.ScheduleType != models.ScheduleTypeActivity {
		sid := schedule.SubjectID
		subjectID = &sid
		if schedule.Subject != nil && schedule.Subject.ID > 0 {
			subjectSummary = &dto.ScheduleSubjectSummary{
				ID:       schedule.Subject.ID,
				Code:     schedule.Subject.Code,
				Name:     schedule.Subject.Name,
				IsActive: schedule.Subject.IsActive,
			}
		}
	}

	var teacherSummary *dto.ScheduleTeacherSummary
	if schedule.Teacher != nil && schedule.Teacher.ID > 0 {
		teacherSummary = &dto.ScheduleTeacherSummary{
			ID:       schedule.Teacher.ID,
			UserID:   schedule.Teacher.UserID,
			Name:     schedule.Teacher.User.Name,
			Email:    schedule.Teacher.User.Email,
			NIP:      schedule.Teacher.NIP,
			IsActive: schedule.Teacher.User.IsActive,
		}
	}

	return dto.ScheduleResponse{
		ID:           schedule.ID,
		Date:         schedule.ScheduleDate.Format(dateFormatYYYYMMDD),
		StartTime:    normalizeTimeDB(schedule.StartTime),
		EndTime:      normalizeTimeDB(schedule.EndTime),
		ClassID:      schedule.ClassID,
		Class:        classSummary,
		SubjectID:    subjectID,
		Subject:      subjectSummary,
		TeacherID:    schedule.TeacherID,
		Teacher:      teacherSummary,
		Type:         schedule.ScheduleType,
		ActivityName: schedule.ActivityName,
		CreatedBy:    schedule.CreatedBy,
		Version:      schedule.Version,
		UpdatedAt:    schedule.UpdatedAt,
	}
}

func mapScheduleWriteError(err error) *utils.AppError {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}
	// 23505 = unique_violation, 23P01 = exclusion_violation (overlap slot)
	if pgErr.Code == "23505" || pgErr.Code == "23P01" {
		switch pgErr.ConstraintName {
		case "uq_schedules_class_slot", "uq_schedules_teacher_slot", "ex_schedules_class_overlap", "ex_schedules_teacher_overlap":
			return utils.ErrScheduleSlotConflict
		}
		return utils.ErrScheduleSlotConflict
	}
	return nil
}
