package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"e-absensi-backend/dto"
	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

// Ambang batas peringatan dini akumulasi hari Alpha dalam 1 tahun ajaran (F5.10).
var alphaWarningThresholds = []int16{
	models.AlphaThresholdWarning3,
	models.AlphaThresholdWarning5,
	models.AlphaThresholdWarning9,
}

// StudentAttendanceService mengelola seluruh proses bisnis EPIC 5 (Student Attendance F5.1-F5.10).
type StudentAttendanceService struct {
	db                 *gorm.DB
	attendances        *repositories.StudentAttendanceRepository
	teacherAttendances *repositories.TeacherAttendanceRepository
	teacherAttService  *TeacherAttendanceService
	schedules          *repositories.ScheduleRepository
	classes            *repositories.ClassRepository
	teachers           *repositories.TeacherRepository
	students           *repositories.StudentRepository
	audit              AuditLogger
	now                func() time.Time
}

// NewStudentAttendanceService membuat instance StudentAttendanceService.
func NewStudentAttendanceService(db *gorm.DB, audit AuditLogger, teacherAttService *TeacherAttendanceService) *StudentAttendanceService {
	if audit == nil {
		audit = NewNoopAuditLogger()
	}
	if teacherAttService == nil {
		teacherAttService = NewTeacherAttendanceService(db, audit)
	}
	return &StudentAttendanceService{
		db:                 db,
		attendances:        repositories.NewStudentAttendanceRepository(db),
		teacherAttendances: repositories.NewTeacherAttendanceRepository(db),
		teacherAttService:  teacherAttService,
		schedules:          repositories.NewScheduleRepository(db),
		classes:            repositories.NewClassRepository(db),
		teachers:           repositories.NewTeacherRepository(db),
		students:           repositories.NewStudentRepository(db),
		audit:              audit,
		now:                time.Now,
	}
}

// =============================================================================
// F5.1 & F5.2 — Form Ceklis Absensi Siswa & Submit Absensi (Guru / Fallback Wali Kelas)
// =============================================================================

// GetAttendanceForm menyiapkan daftar siswa kelas pada suatu jadwal pelajaran,
// melakukan pre-fill otomatis dari pengajuan izin/sakit yang sudah disetujui
// Wali Kelas (F5.7), serta memverifikasi gerbang kehadiran guru (F5.1 / F5.2).
func (s *StudentAttendanceService) GetAttendanceForm(ctx context.Context, actorID int64, actorRoles []string, scheduleID int64) (*dto.ScheduleAttendanceFormResponse, error) {
	_ = s.teacherAttService.SyncExpiredUnconfirmedSchedules(ctx)

	schedule, err := s.schedules.FindByID(scheduleID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrScheduleNotFound
		}
		return nil, utils.ErrInternal
	}
	if schedule.ScheduleType != models.ScheduleTypeLesson {
		return nil, utils.ErrScheduleTypeNotSupported
	}

	teacherAtt, err := s.teacherAttendances.FindByScheduleID(scheduleID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, utils.ErrInternal
	}

	formMode, canSubmitGate, err := s.resolveScheduleFormAccess(actorID, actorRoles, schedule, teacherAtt, true)
	if err != nil {
		return nil, err
	}

	alreadySubmitted, err := s.attendances.HasManualChecklistSubmission(scheduleID)
	if err != nil {
		return nil, utils.ErrInternal
	}

	classStudents, err := s.students.ListByClassID(schedule.ClassID)
	if err != nil {
		return nil, utils.ErrInternal
	}

	existingAttendances, err := s.attendances.FindAttendancesByScheduleID(scheduleID)
	if err != nil {
		return nil, utils.ErrInternal
	}
	attByStudent := make(map[int64]models.StudentAttendance, len(existingAttendances))
	for _, att := range existingAttendances {
		attByStudent[att.StudentID] = att
	}

	leaveRequests, err := s.attendances.FindLeaveRequestsByClassAndDate(schedule.ClassID, schedule.ScheduleDate)
	if err != nil {
		return nil, utils.ErrInternal
	}
	leaveByStudent := make(map[int64]models.LeaveRequest, len(leaveRequests))
	for _, lr := range leaveRequests {
		leaveByStudent[lr.StudentID] = lr
	}

	studentItems := make([]dto.AttendanceFormStudentItem, 0, len(classStudents))
	for _, st := range classStudents {
		item := dto.AttendanceFormStudentItem{
			StudentID: st.ID,
			UserID:    st.UserID,
			Name:      st.User.Name,
			NISN:      st.NISN,
			Status:    models.StudentAttendanceStatusHadir,
		}

		if lr, hasLeave := leaveByStudent[st.ID]; hasLeave {
			lrID := lr.ID
			lrStatus := lr.Status
			item.LeaveRequestID = &lrID
			item.LeaveRequestStatus = &lrStatus
			if lr.Status == models.LeaveRequestStatusDisetujui {
				item.Status = lr.Type // izin atau sakit
				src := models.StudentAttendanceSourceAutoFromLeaveRequest
				item.Source = &src
				item.IsLockedByLeaveRequest = true
			}
		}

		if att, hasAtt := attByStudent[st.ID]; hasAtt {
			attID := att.ID
			src := att.Source
			upd := att.UpdatedAt
			item.AttendanceID = &attID
			item.Status = att.Status
			item.Source = &src
			item.Note = att.Note
			item.UpdatedAt = &upd
			if att.Source == models.StudentAttendanceSourceAutoFromLeaveRequest && item.LeaveRequestStatus != nil && *item.LeaveRequestStatus == models.LeaveRequestStatusDisetujui {
				item.IsLockedByLeaveRequest = true
			}
		}

		studentItems = append(studentItems, item)
	}

	var teacherAttResp *dto.TeacherAttendanceResponse
	if teacherAtt != nil {
		converted := toTeacherAttendanceResponse(teacherAtt)
		teacherAttResp = &converted
	}

	return &dto.ScheduleAttendanceFormResponse{
		Schedule:          toScheduleResponse(schedule),
		TeacherAttendance: teacherAttResp,
		FormMode:          formMode,
		IsAlreadyRecorded: alreadySubmitted,
		CanSubmit:         canSubmitGate && !alreadySubmitted,
		Students:          studentItems,
	}, nil
}

// SubmitScheduleAttendances menyimpan hasil ceklis absensi siswa untuk satu
// jadwal pelajaran oleh Guru Mapel (F5.1) atau Fallback Wali Kelas (F5.2),
// lalu menjalankan perhitungan ulang akumulasi Alpha harian (F5.10).
func (s *StudentAttendanceService) SubmitScheduleAttendances(ctx context.Context, actorID int64, actorRoles []string, scheduleID int64, req dto.SubmitScheduleAttendancesRequest) (*dto.SubmitScheduleAttendancesResponse, error) {
	_ = s.teacherAttService.SyncExpiredUnconfirmedSchedules(ctx)

	if len(req.Items) == 0 {
		return nil, validationError("items", utils.FieldRequired, "items absensi siswa tidak boleh kosong")
	}

	schedule, err := s.schedules.FindByID(scheduleID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrScheduleNotFound
		}
		return nil, utils.ErrInternal
	}
	if schedule.ScheduleType != models.ScheduleTypeLesson {
		return nil, utils.ErrScheduleTypeNotSupported
	}

	teacherAtt, err := s.teacherAttendances.FindByScheduleID(scheduleID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, utils.ErrInternal
	}

	source, canSubmit, err := s.resolveScheduleFormAccess(actorID, actorRoles, schedule, teacherAtt, false)
	if err != nil {
		return nil, err
	}
	if !canSubmit {
		return nil, utils.ErrAttendanceFormNotAvailable
	}

	alreadySubmitted, err := s.attendances.HasManualChecklistSubmission(scheduleID)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if alreadySubmitted {
		return nil, utils.ErrAttendanceAlreadyRecorded
	}

	classObj, err := s.classes.FindByID(schedule.ClassID)
	if err != nil {
		return nil, utils.ErrInternal
	}

	classStudents, err := s.students.ListByClassID(schedule.ClassID)
	if err != nil {
		return nil, utils.ErrInternal
	}
	classStudentMap := make(map[int64]models.Student, len(classStudents))
	for _, st := range classStudents {
		classStudentMap[st.ID] = st
	}

	// Validasi seluruh item yang dikirim.
	submittedByStudent := make(map[int64]dto.SubmitStudentAttendanceItem, len(req.Items))
	for idx, item := range req.Items {
		if _, ok := classStudentMap[item.StudentID]; !ok {
			return nil, utils.ErrStudentOutOfScope.WithField(
				fmt.Sprintf("items[%d].student_id", idx),
				utils.CodeStudentOutOfScope,
				"Siswa tidak terdaftar pada kelas jadwal ini",
			)
		}
		stStatus := strings.ToLower(strings.TrimSpace(item.Status))
		if !isValidStudentAttendanceStatus(stStatus) {
			return nil, validationError(
				fmt.Sprintf("items[%d].status", idx),
				utils.FieldInvalidStatus,
				"status harus hadir, izin, sakit, alpha, atau dispensasi",
			)
		}
		if item.Note != nil && len([]rune(strings.TrimSpace(*item.Note))) > 500 {
			return nil, validationError(
				fmt.Sprintf("items[%d].note", idx),
				utils.FieldTooLong,
				"note maksimal 500 karakter",
			)
		}
		item.Status = stStatus
		submittedByStudent[item.StudentID] = item
	}

	// Muat pengajuan izin/sakit yang sudah disetujui Wali Kelas pada tanggal jadwal ini.
	leaveRequests, err := s.attendances.FindLeaveRequestsByClassAndDate(schedule.ClassID, schedule.ScheduleDate)
	if err != nil {
		return nil, utils.ErrInternal
	}
	approvedLeaveByStudent := make(map[int64]models.LeaveRequest)
	for _, lr := range leaveRequests {
		if lr.Status == models.LeaveRequestStatusDisetujui {
			approvedLeaveByStudent[lr.StudentID] = lr
		}
	}

	now := s.now()
	affectedStudentIDs := make([]int64, 0, len(classStudents))

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txAtt := s.attendances.WithTx(tx)

		// Pastikan seluruh siswa di kelas jadwal ini tercatat (default hadir bila tidak disebut eksplisit),
		// sementara siswa dengan izin/sakit disetujui tetap terkunci sesuai pengajuannya (F5.1 & F5.7).
		for _, st := range classStudents {
			status := models.StudentAttendanceStatusHadir
			recSource := source
			var notePtr *string

			if subItem, ok := submittedByStudent[st.ID]; ok {
				status = subItem.Status
				if subItem.Note != nil {
					trimmed := strings.TrimSpace(*subItem.Note)
					if trimmed != "" {
						notePtr = &trimmed
					}
				}
			}

			if approvedLR, hasApproved := approvedLeaveByStudent[st.ID]; hasApproved {
				status = approvedLR.Type
				recSource = models.StudentAttendanceSourceAutoFromLeaveRequest
			}

			rec := &models.StudentAttendance{
				ScheduleID: schedule.ID,
				StudentID:  st.ID,
				Status:     status,
				Source:     recSource,
				RecordedBy: actorID,
				Note:       notePtr,
				UpdatedAt:  now,
			}
			if err := txAtt.UpsertStudentAttendance(rec); err != nil {
				return err
			}
			affectedStudentIDs = append(affectedStudentIDs, st.ID)
		}

		// Jika submitted hanya berisi siswa yang tidak ada di classStudents (tidak mungkin karena sudah dicek),
		// atau jika classStudents kosong tapi req.Items ada (sudah tertolak di atas).
		for _, studentID := range affectedStudentIDs {
			if err := s.recomputeStudentAlphaOnDateTx(ctx, tx, studentID, schedule.ClassID, classObj.AcademicYearID, schedule.ScheduleDate, now); err != nil {
				return err
			}
		}

		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"schedule_id":    schedule.ID,
			"class_id":       schedule.ClassID,
			"date":           schedule.ScheduleDate.Format(dateFormatYYYYMMDD),
			"source":         source,
			"recorded_count": len(affectedStudentIDs),
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeStudentAttendance,
			TargetID:   schedule.ID,
			Action:     ActionAttendanceRecorded,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, utils.ErrInternal
	}

	savedRecords, err := s.attendances.FindAttendancesByScheduleID(schedule.ID)
	if err != nil {
		return nil, utils.ErrInternal
	}
	respItems := make([]dto.StudentAttendanceResponse, 0, len(savedRecords))
	for i := range savedRecords {
		respItems = append(respItems, toStudentAttendanceResponse(&savedRecords[i]))
	}

	return &dto.SubmitScheduleAttendancesResponse{
		ScheduleID:    schedule.ID,
		Source:        source,
		RecordedCount: len(respItems),
		Items:         respItems,
	}, nil
}

// resolveScheduleFormAccess memeriksa gerbang antara Epic 6 dan Epic 5:
//   - F5.1 (Guru Mapel): hanya terbuka bila teacher_attendances untuk schedule_id
//     sudah berstatus hadir/guru_pengganti DAN confirmed_teacher_id == teacher.ID pemanggil.
//   - F5.2 (Wali Kelas Fallback): hanya terbuka untuk Wali Kelas dari schedule.ClassID
//     SETELAH jam selesai pelajaran lewat DAN guru tidak hadir (belum ada konfirmasi hadir/pengganti).
func (s *StudentAttendanceService) resolveScheduleFormAccess(
	actorID int64,
	actorRoles []string,
	schedule *models.Schedule,
	teacherAtt *models.TeacherAttendance,
	allowReadOnlyAdmin bool,
) (string, bool, error) {
	isConfirmedPresent := teacherAtt != nil &&
		(teacherAtt.Status == models.TeacherAttendanceStatusHadir || teacherAtt.Status == models.TeacherAttendanceStatusGuruPengganti) &&
		teacherAtt.ConfirmedTeacherID != nil

	// 1. Cek apakah pemanggil adalah Guru Mapel yang mengonfirmasi kehadiran pada jadwal ini (F5.1).
	if hasAnyRole(actorRoles, models.RoleGuruMapel) {
		teacher, err := s.teachers.FindByUserID(actorID)
		if err == nil && isConfirmedPresent && *teacherAtt.ConfirmedTeacherID == teacher.ID {
			return models.StudentAttendanceSourceCeklisGuru, true, nil
		}
	}

	// 2. Cek apakah pemanggil adalah Wali Kelas untuk F5.2 Fallback.
	if hasAnyRole(actorRoles, models.RoleWaliKelas) {
		waliClassIDs, err := s.classes.FindIDsByWaliKelas(actorID)
		if err != nil {
			return "", false, utils.ErrInternal
		}
		if containsInt64(waliClassIDs, schedule.ClassID) {
			if isConfirmedPresent {
				return "", false, utils.ErrAttendanceFormNotAvailable.WithMessage("Guru pengampu sudah mengonfirmasi kehadiran; input absensi dilakukan oleh guru tersebut")
			}
			if !s.hasScheduleEnded(schedule) {
				return "", false, utils.ErrAttendanceFormNotAvailable.WithMessage("Input absensi fallback Wali Kelas baru terbuka setelah jam pelajaran selesai tanpa kehadiran guru")
			}
			return models.StudentAttendanceSourceFallbackWaliKelas, true, nil
		}
	}

	// 3. Admin / Waka Kurikulum boleh melihat form dalam mode read-only bila allowReadOnlyAdmin = true.
	if allowReadOnlyAdmin && hasAnyRole(actorRoles, models.RoleAdmin, models.RoleWakaKurikulum) {
		mode := models.StudentAttendanceSourceCeklisGuru
		if !isConfirmedPresent {
			mode = models.StudentAttendanceSourceFallbackWaliKelas
		}
		return mode, false, nil
	}

	return "", false, utils.ErrAttendanceFormNotAvailable
}

func (s *StudentAttendanceService) hasScheduleEnded(schedule *models.Schedule) bool {
	now := s.now().In(jakartaLocation())
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakartaLocation())
	schedDate := time.Date(schedule.ScheduleDate.Year(), schedule.ScheduleDate.Month(), schedule.ScheduleDate.Day(), 0, 0, 0, 0, jakartaLocation())

	if schedDate.Before(todayDate) {
		return true
	}
	if schedDate.After(todayDate) {
		return false
	}
	nowHHMM := now.Format("15:04")
	endHHMM := normalizeTimeDB(schedule.EndTime)
	return nowHHMM > endHHMM
}

// =============================================================================
// F5.3 — Ubah Status Absensi Siswa (PATCH /api/v1/attendances/{id})
// =============================================================================

// UpdateAttendance mengubah status absensi siswa pada satu jadwal dengan
// optimistic locking updated_at, validasi cakupan role & batas hari, serta
// menghitung ulang akumulasi Alpha harian (F5.3 & F5.10).
func (s *StudentAttendanceService) UpdateAttendance(ctx context.Context, actorID int64, actorRoles []string, id int64, req dto.UpdateStudentAttendanceRequest) (*dto.StudentAttendanceResponse, error) {
	if req.UpdatedAt == nil {
		return nil, validationError("updated_at", utils.FieldRequired, "updated_at wajib diisi")
	}

	targetStatus := strings.ToLower(strings.TrimSpace(req.Status))
	if !isValidStudentAttendanceStatus(targetStatus) {
		return nil, validationError("status", utils.FieldInvalidStatus, "status harus hadir, izin, sakit, alpha, atau dispensasi")
	}

	var notePtr *string
	if req.Note != nil {
		trimmed := strings.TrimSpace(*req.Note)
		if len([]rune(trimmed)) > 500 {
			return nil, validationError("note", utils.FieldTooLong, "note maksimal 500 karakter")
		}
		if trimmed != "" {
			notePtr = &trimmed
		}
	}

	now := s.now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txAtt := s.attendances.WithTx(tx)
		txSchedules := s.schedules.WithTx(tx)

		locked, err := txAtt.FindAttendanceByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrAttendanceNotFound
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

		if err := s.authorizeAttendanceEdit(actorID, actorRoles, locked, schedule); err != nil {
			return err
		}

		updates := map[string]interface{}{
			"status":      targetStatus,
			"source":      models.StudentAttendanceSourceManualEdit,
			"recorded_by": actorID,
			"updated_at":  now,
		}
		if req.Note != nil {
			updates["note"] = notePtr
		}

		rows, err := txAtt.UpdateAttendance(id, locked.UpdatedAt, updates)
		if err != nil {
			return err
		}
		if rows == 0 {
			return utils.ErrStaleData
		}

		if err := s.recomputeStudentAlphaOnDateTx(ctx, tx, locked.StudentID, locked.ClassID, locked.AcademicYearID, schedule.ScheduleDate, now); err != nil {
			return err
		}

		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"id":            id,
			"schedule_id":   locked.ScheduleID,
			"student_id":    locked.StudentID,
			"class_id":      locked.ClassID,
			"status_before": locked.Status,
			"status_after":  targetStatus,
			"source_before": locked.Source,
			"source_after":  models.StudentAttendanceSourceManualEdit,
			"note_before":   locked.Note,
			"note_after":    updates["note"],
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeStudentAttendance,
			TargetID:   id,
			Action:     ActionAttendanceUpdated,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, utils.ErrInternal
	}

	updated, err := s.attendances.FindAttendanceByID(id)
	if err != nil {
		return nil, utils.ErrInternal
	}
	resp := toStudentAttendanceResponse(updated)
	return &resp, nil
}

func (s *StudentAttendanceService) authorizeAttendanceEdit(actorID int64, actorRoles []string, rec *models.StudentAttendance, schedule *models.Schedule) error {
	// Admin & Waka Kurikulum dapat mengedit seluruh sekolah tanpa batas hari.
	if hasAnyRole(actorRoles, models.RoleAdmin, models.RoleWakaKurikulum) {
		return nil
	}

	// Wali Kelas dapat mengedit absensi siswa di kelas perwaliannya dalam semester aktif.
	if hasAnyRole(actorRoles, models.RoleWaliKelas) {
		waliClassIDs, err := s.classes.FindIDsByWaliKelas(actorID)
		if err != nil {
			return utils.ErrInternal
		}
		if containsInt64(waliClassIDs, rec.ClassID) {
			return nil
		}
	}

	// Guru Mapel hanya dapat mengedit absensi pada jadwal yang diampunya DAN pada hari yang sama.
	if hasAnyRole(actorRoles, models.RoleGuruMapel) {
		teacher, err := s.teachers.FindByUserID(actorID)
		if err != nil {
			return utils.ErrForbidden
		}
		isOwner := schedule.TeacherID == teacher.ID
		if !isOwner {
			ta, err := s.teacherAttendances.FindByScheduleID(schedule.ID)
			if err == nil && ta != nil && ta.ConfirmedTeacherID != nil && *ta.ConfirmedTeacherID == teacher.ID {
				isOwner = true
			}
		}
		if !isOwner {
			return utils.ErrForbidden.WithMessage("Anda hanya dapat mengubah absensi pada jadwal yang Anda ampu")
		}
		now := s.now().In(jakartaLocation())
		todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakartaLocation())
		if !sameDate(schedule.ScheduleDate, todayDate) {
			return utils.ErrForbidden.WithMessage("Guru Mapel hanya dapat mengubah status absensi pada hari yang sama")
		}
		return nil
	}

	return utils.ErrForbidden
}

// =============================================================================
// F5.4 & F5.5 — Lihat Absensi, Rekap, & Export Excel
// =============================================================================

// ListAttendances mengambil daftar absensi siswa berhalaman beserta ringkasan
// statistik hariannya dengan scoping sesuai role (F5.4).
func (s *StudentAttendanceService) ListAttendances(ctx context.Context, actorID int64, actorRoles []string, filter dto.StudentAttendanceListFilter) (*dto.StudentAttendanceListResponse, error) {
	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	dateFrom, dateTo, err := s.resolveStudentAttendanceDateRange(filter.Date, filter.DateFrom, filter.DateTo, filter.Preset)
	if err != nil {
		return nil, err
	}

	status := strings.ToLower(strings.TrimSpace(filter.Status))
	if status != "" && !isValidStudentAttendanceStatus(status) {
		return nil, validationError("status", utils.FieldInvalidStatus, "status harus hadir, izin, sakit, alpha, atau dispensasi")
	}

	source := strings.ToLower(strings.TrimSpace(filter.Source))
	if source != "" &&
		source != models.StudentAttendanceSourceCeklisGuru &&
		source != models.StudentAttendanceSourceFallbackWaliKelas &&
		source != models.StudentAttendanceSourceAutoFromLeaveRequest &&
		source != models.StudentAttendanceSourceManualEdit {
		return nil, validationError("source", utils.FieldInvalidStatus, "source tidak valid")
	}

	params := repositories.StudentAttendanceListParams{
		Page:       page,
		Limit:      limit,
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		ClassID:    filter.ClassID,
		ScheduleID: filter.ScheduleID,
		StudentID:  filter.StudentID,
		SubjectID:  filter.SubjectID,
		Status:     status,
		Source:     source,
	}

	if err := s.applyStudentAttendanceScoping(actorID, actorRoles, &params); err != nil {
		return nil, err
	}

	items, total, counts, err := s.attendances.ListAttendances(params)
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.StudentAttendanceResponse, 0, len(items))
	for i := range items {
		respItems = append(respItems, toStudentAttendanceResponse(&items[i]))
	}

	totalAll := counts.Hadir + counts.Izin + counts.Sakit + counts.Alpha + counts.Dispensasi
	return &dto.StudentAttendanceListResponse{
		Items: respItems,
		Summary: dto.StudentAttendanceSummaryStats{
			Total:      totalAll,
			Hadir:      counts.Hadir,
			Izin:       counts.Izin,
			Sakit:      counts.Sakit,
			Alpha:      counts.Alpha,
			Dispensasi: counts.Dispensasi,
		},
		DateFrom: dateFrom.Format(dateFormatYYYYMMDD),
		DateTo:   dateTo.Format(dateFormatYYYYMMDD),
		Meta:     buildPaginationMeta(page, limit, total),
	}, nil
}

// RecapAttendances mengambil rekap absensi per siswa dalam rentang tanggal
// beserta total hari Alpha dan ambang peringatan dini (F5.5 & F5.10).
func (s *StudentAttendanceService) RecapAttendances(ctx context.Context, actorID int64, actorRoles []string, filter dto.StudentAttendanceListFilter) (*dto.StudentAttendanceRecapResponse, error) {
	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	dateFrom, dateTo, err := s.resolveStudentAttendanceDateRange(filter.Date, filter.DateFrom, filter.DateTo, filter.Preset)
	if err != nil {
		return nil, err
	}

	params := repositories.StudentAttendanceListParams{
		Page:      page,
		Limit:     limit,
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		ClassID:   filter.ClassID,
		StudentID: filter.StudentID,
	}
	if err := s.applyStudentAttendanceScoping(actorID, actorRoles, &params); err != nil {
		return nil, err
	}

	rows, total, counts, err := s.attendances.RecapByStudents(params)
	if err != nil {
		return nil, utils.ErrInternal
	}

	items := make([]dto.StudentAttendanceRecapItem, 0, len(rows))
	for _, row := range rows {
		totalSessions := row.HadirSessions + row.IzinSessions + row.SakitSessions + row.AlphaSessions + row.DispensasiSessions
		var rate float64
		if totalSessions > 0 {
			rate = float64(row.HadirSessions+row.DispensasiSessions) * 100.0 / float64(totalSessions)
		}
		items = append(items, dto.StudentAttendanceRecapItem{
			StudentID:                  row.StudentID,
			UserID:                     row.UserID,
			StudentName:                row.StudentName,
			NISN:                       row.NISN,
			ClassID:                    row.ClassID,
			ClassName:                  row.ClassName,
			HadirSessions:              row.HadirSessions,
			IzinSessions:               row.IzinSessions,
			SakitSessions:              row.SakitSessions,
			AlphaSessions:              row.AlphaSessions,
			DispensasiSessions:         row.DispensasiSessions,
			TotalSessions:              totalSessions,
			AttendanceRatePercent:      roundTwoDecimals(rate),
			AlphaDaysInRange:           row.AlphaDaysInRange,
			TotalAlphaDaysAcademicYear: row.TotalAlphaDaysAcademicYear,
			LatestWarningThreshold:     row.LatestWarningThreshold,
		})
	}

	totalAll := counts.Hadir + counts.Izin + counts.Sakit + counts.Alpha + counts.Dispensasi
	return &dto.StudentAttendanceRecapResponse{
		Items: items,
		Summary: dto.StudentAttendanceSummaryStats{
			Total:      totalAll,
			Hadir:      counts.Hadir,
			Izin:       counts.Izin,
			Sakit:      counts.Sakit,
			Alpha:      counts.Alpha,
			Dispensasi: counts.Dispensasi,
		},
		DateFrom: dateFrom.Format(dateFormatYYYYMMDD),
		DateTo:   dateTo.Format(dateFormatYYYYMMDD),
		Meta:     buildPaginationMeta(page, limit, total),
	}, nil
}

// ExportAttendances mengekspor rekap absensi siswa ke file Excel (.xlsx) dan
// mencatat event ATTENDANCE_EXPORTED ke activity_logs (F5.5).
func (s *StudentAttendanceService) ExportAttendances(ctx context.Context, actorID int64, actorRoles []string, filter dto.StudentAttendanceListFilter) ([]byte, string, error) {
	dateFrom, dateTo, err := s.resolveStudentAttendanceDateRange(filter.Date, filter.DateFrom, filter.DateTo, filter.Preset)
	if err != nil {
		return nil, "", err
	}

	params := repositories.StudentAttendanceListParams{
		Page:      1,
		Limit:     0, // tanpa batas halaman untuk export
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		ClassID:   filter.ClassID,
		StudentID: filter.StudentID,
	}
	if err := s.applyStudentAttendanceScoping(actorID, actorRoles, &params); err != nil {
		return nil, "", err
	}

	rows, _, _, err := s.attendances.RecapByStudents(params)
	if err != nil {
		return nil, "", utils.ErrInternal
	}

	excelRows := make([][]string, 0, len(rows)+1)
	excelRows = append(excelRows, []string{
		"No",
		"NISN",
		"Nama Siswa",
		"Kelas",
		"Sesi Hadir",
		"Sesi Izin",
		"Sesi Sakit",
		"Sesi Alpha",
		"Sesi Dispensasi",
		"Total Sesi",
		"Persentase Kehadiran (%)",
		"Hari Alpha (Periode)",
		"Total Hari Alpha (Tahun Ajaran)",
		"Status Peringatan Dini",
	})

	for idx, rRow := range rows {
		totalSessions := rRow.HadirSessions + rRow.IzinSessions + rRow.SakitSessions + rRow.AlphaSessions + rRow.DispensasiSessions
		var rate float64
		if totalSessions > 0 {
			rate = float64(rRow.HadirSessions+rRow.DispensasiSessions) * 100.0 / float64(totalSessions)
		}
		warningLabel := "-"
		if rRow.LatestWarningThreshold != nil {
			warningLabel = fmt.Sprintf("Peringatan %d Hari Alpha", *rRow.LatestWarningThreshold)
		}

		excelRows = append(excelRows, []string{
			fmt.Sprintf("%d", idx+1),
			rRow.NISN,
			rRow.StudentName,
			rRow.ClassName,
			fmt.Sprintf("%d", rRow.HadirSessions),
			fmt.Sprintf("%d", rRow.IzinSessions),
			fmt.Sprintf("%d", rRow.SakitSessions),
			fmt.Sprintf("%d", rRow.AlphaSessions),
			fmt.Sprintf("%d", rRow.DispensasiSessions),
			fmt.Sprintf("%d", totalSessions),
			fmt.Sprintf("%.2f", roundTwoDecimals(rate)),
			fmt.Sprintf("%d", rRow.AlphaDaysInRange),
			fmt.Sprintf("%d", rRow.TotalAlphaDaysAcademicYear),
			warningLabel,
		})
	}

	content, err := utils.BuildXLSX("Rekap Absensi Siswa", excelRows)
	if err != nil {
		return nil, "", utils.ErrInternal
	}

	_ = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"date_from":     dateFrom.Format(dateFormatYYYYMMDD),
			"date_to":       dateTo.Format(dateFormatYYYYMMDD),
			"class_id":      filter.ClassID,
			"student_count": len(rows),
		})
		if err != nil {
			return err
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeStudentAttendance,
			TargetID:   0,
			Action:     ActionAttendanceExported,
			Snapshot:   snapshot,
		})
	})

	filename := fmt.Sprintf("rekap_absensi_siswa_%s_sd_%s.xlsx",
		dateFrom.Format(dateFormatYYYYMMDD),
		dateTo.Format(dateFormatYYYYMMDD),
	)
	return content, filename, nil
}

// =============================================================================
// F5.6, F5.7, F5.9 — Pengajuan & Verifikasi Izin/Sakit Siswa
// =============================================================================

// SubmitLeaveRequest mencatat pengajuan izin/sakit baru oleh Siswa atau Siswa Inti
// untuk dirinya sendiri pada hari ini sebelum pukul 09:00 WIB (F5.6).
func (s *StudentAttendanceService) SubmitLeaveRequest(ctx context.Context, actorID int64, req dto.SubmitLeaveRequestPayload) (*dto.LeaveRequestResponse, error) {
	student, err := s.students.FindByUserID(actorID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrForbidden.WithMessage("Akun Anda tidak terdaftar sebagai siswa")
		}
		return nil, utils.ErrInternal
	}
	if student.ClassID == nil {
		return nil, utils.ErrForbidden.WithMessage("Akun siswa belum ditempatkan pada kelas aktif")
	}

	leaveType := strings.ToLower(strings.TrimSpace(req.Type))
	if leaveType != models.LeaveRequestTypeIzin && leaveType != models.LeaveRequestTypeSakit {
		return nil, validationError("type", utils.FieldInvalidStatus, "type harus izin atau sakit")
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, validationError("reason", utils.FieldRequired, "Alasan izin/sakit wajib diisi")
	}
	if len([]rune(reason)) > 500 {
		return nil, validationError("reason", utils.FieldTooLong, "Alasan izin/sakit maksimal 500 karakter")
	}

	evidenceURL := strings.TrimSpace(req.EvidenceFileURL)
	if evidenceURL == "" {
		return nil, utils.ErrEvidenceFileRequired.WithField("evidence_file", utils.CodeEvidenceFileRequired, "Bukti foto atau dokumen PDF wajib dilampirkan")
	}
	if len(evidenceURL) > 1000 {
		return nil, validationError("evidence_file_url", utils.FieldTooLong, "URL file bukti maksimal 1000 karakter")
	}

	now := s.now().In(jakartaLocation())
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakartaLocation())

	targetDate := todayDate
	if strings.TrimSpace(req.Date) != "" {
		parsedDate, err := parseDateField("date", req.Date)
		if err != nil {
			return nil, err
		}
		if !sameDate(parsedDate, todayDate) {
			return nil, validationError("date", utils.FieldInvalidDate, "Pengajuan izin/sakit hanya dapat dilakukan untuk tanggal hari ini")
		}
	}

	// Batas waktu pengajuan pukul 09:00 WIB (sebelum 09:00:00).
	if now.Hour() >= 9 {
		return nil, utils.ErrLeaveRequestDeadlinePassed
	}

	exists, err := s.attendances.ExistsLeaveRequestForStudentDate(student.ID, targetDate)
	if err != nil {
		return nil, utils.ErrInternal
	}
	if exists {
		return nil, utils.ErrLeaveRequestAlreadyActive
	}

	record := &models.LeaveRequest{
		StudentID:       student.ID,
		ClassID:         *student.ClassID,
		Type:            leaveType,
		Date:            targetDate,
		Reason:          reason,
		EvidenceFileURL: evidenceURL,
		Status:          models.LeaveRequestStatusDraft,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txAtt := s.attendances.WithTx(tx)
		if err := txAtt.CreateLeaveRequest(record); err != nil {
			return err
		}

		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"id":                record.ID,
			"student_id":        record.StudentID,
			"class_id":          record.ClassID,
			"type":              record.Type,
			"date":              record.Date.Format(dateFormatYYYYMMDD),
			"status":            record.Status,
			"evidence_file_url": record.EvidenceFileURL,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeLeaveRequest,
			TargetID:   record.ID,
			Action:     ActionLeaveRequestSubmitted,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		if isUniqueViolation(err) {
			return nil, utils.ErrLeaveRequestAlreadyActive
		}
		return nil, utils.ErrInternal
	}

	full, err := s.attendances.FindLeaveRequestByID(record.ID)
	if err != nil {
		return nil, utils.ErrInternal
	}
	resp := toLeaveRequestResponse(full, false)
	return &resp, nil
}

// ApproveLeaveRequest menyetujui pengajuan izin/sakit (status = disetujui) oleh
// Wali Kelas, otomatis mengisi absensi siswa pada seluruh jadwal pelajaran kelas
// di tanggal tersebut dengan source = auto_from_leave_request, dan menghitung
// ulang Alpha harian (F5.7 & F5.10).
func (s *StudentAttendanceService) ApproveLeaveRequest(ctx context.Context, actorID int64, actorRoles []string, id int64) (*dto.LeaveRequestResponse, error) {
	return s.reviewLeaveRequest(ctx, actorID, actorRoles, id, models.LeaveRequestStatusDisetujui)
}

// RejectLeaveRequest menolak pengajuan izin/sakit (status = ditolak) oleh Wali Kelas,
// otomatis mengisi absensi siswa pada seluruh jadwal pelajaran kelas di tanggal
// tersebut menjadi alpha dengan source = auto_from_leave_request, dan menghitung
// ulang Alpha harian (F5.7 & F5.10).
func (s *StudentAttendanceService) RejectLeaveRequest(ctx context.Context, actorID int64, actorRoles []string, id int64) (*dto.LeaveRequestResponse, error) {
	return s.reviewLeaveRequest(ctx, actorID, actorRoles, id, models.LeaveRequestStatusDitolak)
}

func (s *StudentAttendanceService) reviewLeaveRequest(ctx context.Context, actorID int64, actorRoles []string, id int64, decision string) (*dto.LeaveRequestResponse, error) {
	waliClassIDs, err := s.classes.FindIDsByWaliKelas(actorID)
	if err != nil {
		return nil, utils.ErrInternal
	}

	now := s.now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txAtt := s.attendances.WithTx(tx)
		txClasses := s.classes.WithTx(tx)

		locked, err := txAtt.FindLeaveRequestByIDForUpdate(id)
		if err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return utils.ErrLeaveRequestNotFound
			}
			return utils.ErrInternal
		}

		// Hanya Wali Kelas dari kelas pengajuan (atau Admin dalam kondisi darurat bila punya permission) yang boleh memverifikasi.
		if !containsInt64(waliClassIDs, locked.ClassID) && !hasAnyRole(actorRoles, models.RoleAdmin) {
			return utils.ErrForbidden.WithMessage("Anda hanya dapat memverifikasi pengajuan izin/sakit siswa di kelas perwalian Anda")
		}

		if locked.Status != models.LeaveRequestStatusDraft {
			return utils.ErrLeaveRequestAlreadyReviewed
		}

		if err := txAtt.UpdateLeaveRequestStatus(id, decision, actorID, now); err != nil {
			return err
		}

		classObj, err := txClasses.FindByID(locked.ClassID)
		if err != nil {
			return utils.ErrInternal
		}

		lessonSchedules, err := txAtt.FindLessonSchedulesByClassAndDate(locked.ClassID, locked.Date)
		if err != nil {
			return utils.ErrInternal
		}

		attendanceStatus := locked.Type // izin atau sakit jika disetujui
		if decision == models.LeaveRequestStatusDitolak {
			attendanceStatus = models.StudentAttendanceStatusAlpha
		}

		for _, sched := range lessonSchedules {
			rec := &models.StudentAttendance{
				ScheduleID: sched.ID,
				StudentID:  locked.StudentID,
				Status:     attendanceStatus,
				Source:     models.StudentAttendanceSourceAutoFromLeaveRequest,
				RecordedBy: actorID,
				UpdatedAt:  now,
			}
			if err := txAtt.UpsertStudentAttendance(rec); err != nil {
				return err
			}
		}

		if err := s.recomputeStudentAlphaOnDateTx(ctx, tx, locked.StudentID, locked.ClassID, classObj.AcademicYearID, locked.Date, now); err != nil {
			return err
		}

		action := ActionLeaveRequestApproved
		if decision == models.LeaveRequestStatusDitolak {
			action = ActionLeaveRequestRejected
		}

		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"id":                      id,
			"student_id":              locked.StudentID,
			"class_id":                locked.ClassID,
			"type":                    locked.Type,
			"date":                    locked.Date.Format(dateFormatYYYYMMDD),
			"status_before":           locked.Status,
			"status_after":            decision,
			"auto_filled_schedules":   len(lessonSchedules),
			"auto_filled_with_status": attendanceStatus,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypeLeaveRequest,
			TargetID:   id,
			Action:     action,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, utils.ErrInternal
	}

	updated, err := s.attendances.FindLeaveRequestByID(id)
	if err != nil {
		return nil, utils.ErrInternal
	}
	resp := toLeaveRequestResponse(updated, false)
	return &resp, nil
}

// ListLeaveRequests mengambil daftar pengajuan izin/sakit sesuai cakupan role.
// Untuk Guru Mapel dan Siswa Inti yang melihat data kelas (bukan miliknya sendiri),
// field sensitif reason dan evidence_file_url disembunyikan sesuai privasi F5.9.
func (s *StudentAttendanceService) ListLeaveRequests(ctx context.Context, actorID int64, actorRoles []string, filter dto.LeaveRequestListFilter) (*dto.LeaveRequestListResponse, error) {
	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	dateFrom, dateTo, err := s.resolveOptionalDateFilter(filter.Date, filter.DateFrom, filter.DateTo)
	if err != nil {
		return nil, err
	}

	tp := strings.ToLower(strings.TrimSpace(filter.Type))
	if tp != "" && tp != models.LeaveRequestTypeIzin && tp != models.LeaveRequestTypeSakit {
		return nil, validationError("type", utils.FieldInvalidStatus, "type harus izin atau sakit")
	}

	st := strings.ToLower(strings.TrimSpace(filter.Status))
	if st != "" && st != models.LeaveRequestStatusDraft && st != models.LeaveRequestStatusDisetujui && st != models.LeaveRequestStatusDitolak {
		return nil, validationError("status", utils.FieldInvalidStatus, "status harus draft, disetujui, atau ditolak")
	}

	params := repositories.LeaveRequestListParams{
		Page:      page,
		Limit:     limit,
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		ClassID:   filter.ClassID,
		StudentID: filter.StudentID,
		Type:      tp,
		Status:    st,
	}

	maskSensitive, myStudentID, err := s.applyLeaveRequestScoping(actorID, actorRoles, &params)
	if err != nil {
		return nil, err
	}

	items, total, err := s.attendances.ListLeaveRequests(params)
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.LeaveRequestResponse, 0, len(items))
	for i := range items {
		hideDetails := maskSensitive && (myStudentID == nil || items[i].StudentID != *myStudentID)
		respItems = append(respItems, toLeaveRequestResponse(&items[i], hideDetails))
	}

	return &dto.LeaveRequestListResponse{
		Items: respItems,
		Meta:  buildPaginationMeta(page, limit, total),
	}, nil
}

// GetClassTodayLeaveSummary mengambil ringkasan status izin/sakit kelas hari ini
// untuk Siswa Inti, Guru Mapel, Wali Kelas, dan Admin/Waka tanpa mengekspos alasan
// pribadi maupun file bukti siswa (F5.9).
func (s *StudentAttendanceService) GetClassTodayLeaveSummary(ctx context.Context, actorID int64, actorRoles []string, classID *int64, dateStr string) (*dto.ClassTodayLeaveSummaryResponse, error) {
	now := s.now().In(jakartaLocation())
	targetDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakartaLocation())
	if strings.TrimSpace(dateStr) != "" {
		parsed, err := parseDateField("date", dateStr)
		if err != nil {
			return nil, err
		}
		targetDate = parsed
	}

	params := repositories.LeaveRequestListParams{
		Page:     1,
		Limit:    200,
		DateFrom: targetDate,
		DateTo:   targetDate,
		ClassID:  classID,
	}

	if hasAnyRole(actorRoles, models.RoleAdmin, models.RoleWakaKurikulum, models.RoleWakaKesiswaan, models.RoleGuruMapel) {
		// Boleh melihat kelas yang diminta (atau semua kelas pada tanggal tersebut).
	} else if hasAnyRole(actorRoles, models.RoleWaliKelas) {
		waliClassIDs, err := s.classes.FindIDsByWaliKelas(actorID)
		if err != nil {
			return nil, utils.ErrInternal
		}
		if classID != nil && !containsInt64(waliClassIDs, *classID) {
			return nil, utils.ErrForbidden
		}
		params.RestrictToClassIDs = waliClassIDs
	} else if hasAnyRole(actorRoles, models.RoleSiswaInti) {
		student, err := s.students.FindByUserID(actorID)
		if err != nil || student.ClassID == nil {
			return nil, utils.ErrForbidden
		}
		if classID != nil && *classID != *student.ClassID {
			return nil, utils.ErrForbidden
		}
		params.RestrictToClassIDs = []int64{*student.ClassID}
	} else {
		return nil, utils.ErrForbidden
	}

	items, _, err := s.attendances.ListLeaveRequests(params)
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.LeaveRequestResponse, 0, len(items))
	var totalDraft, totalIzin, totalSakit int
	for i := range items {
		if items[i].Status == models.LeaveRequestStatusDraft {
			totalDraft++
		}
		if items[i].Type == models.LeaveRequestTypeIzin {
			totalIzin++
		} else if items[i].Type == models.LeaveRequestTypeSakit {
			totalSakit++
		}
		// Selalu sembunyikan reason & evidence_file_url pada widget publik kelas F5.9.
		respItems = append(respItems, toLeaveRequestResponse(&items[i], true))
	}

	return &dto.ClassTodayLeaveSummaryResponse{
		Date:       targetDate.Format(dateFormatYYYYMMDD),
		TotalItems: len(respItems),
		TotalDraft: totalDraft,
		TotalIzin:  totalIzin,
		TotalSakit: totalSakit,
		Items:      respItems,
	}, nil
}

// =============================================================================
// F5.8 — Upload & Lihat Surat Izin Pulang (Tatib) & Surat Dispensasi
// =============================================================================

// UploadPermissionLetter menyimpan bukti surat izin pulang dari Tatib atau
// surat dispensasi yang diunggah oleh Siswa Inti untuk siswa di kelasnya (F5.8).
func (s *StudentAttendanceService) UploadPermissionLetter(ctx context.Context, actorID int64, req dto.UploadPermissionLetterPayload) (*dto.PermissionLetterResponse, error) {
	if req.StudentID <= 0 {
		return nil, validationError("student_id", utils.FieldRequired, "student_id wajib diisi")
	}

	letterType := strings.ToLower(strings.TrimSpace(req.Type))
	if letterType != models.PermissionLetterTypePulangTatib && letterType != models.PermissionLetterTypeDispensasi {
		return nil, validationError("type", utils.FieldInvalidStatus, "type harus pulang_tatib atau dispensasi")
	}

	evidenceURL := strings.TrimSpace(req.EvidenceFileURL)
	if evidenceURL == "" {
		return nil, utils.ErrEvidenceFileRequired.WithField("evidence_file", utils.CodeEvidenceFileRequired, "File bukti surat izin wajib dilampirkan")
	}
	if len(evidenceURL) > 1000 {
		return nil, validationError("evidence_file_url", utils.FieldTooLong, "URL file bukti maksimal 1000 karakter")
	}

	var notePtr *string
	if req.Note != nil {
		trimmed := strings.TrimSpace(*req.Note)
		if len([]rune(trimmed)) > 500 {
			return nil, validationError("note", utils.FieldTooLong, "note maksimal 500 karakter")
		}
		if trimmed != "" {
			notePtr = &trimmed
		}
	}

	uploaderStudent, err := s.students.FindByUserID(actorID)
	if err != nil || uploaderStudent.ClassID == nil {
		return nil, utils.ErrForbidden.WithMessage("Akun Siswa Inti belum ditempatkan pada kelas aktif")
	}

	targetStudent, err := s.students.FindByID(req.StudentID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, utils.ErrStudentNotFound
		}
		return nil, utils.ErrInternal
	}
	if targetStudent.ClassID == nil || *targetStudent.ClassID != *uploaderStudent.ClassID {
		return nil, utils.ErrStudentOutOfScope.WithMessage("Siswa Inti hanya dapat mengunggah surat izin untuk siswa di kelas yang sama")
	}

	record := &models.PermissionLetter{
		StudentID:       targetStudent.ID,
		Type:            letterType,
		UploadedBy:      actorID,
		EvidenceFileURL: evidenceURL,
		Note:            notePtr,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txAtt := s.attendances.WithTx(tx)
		if err := txAtt.CreatePermissionLetter(record); err != nil {
			return err
		}

		snapshot, err := marshalAuditSnapshot(map[string]interface{}{
			"id":                record.ID,
			"student_id":        record.StudentID,
			"class_id":          *targetStudent.ClassID,
			"type":              record.Type,
			"uploaded_by":       record.UploadedBy,
			"evidence_file_url": record.EvidenceFileURL,
			"note":              record.Note,
		})
		if err != nil {
			return utils.ErrInternal
		}
		return s.audit.LogTx(ctx, tx, AuditEvent{
			ActorID:    actorID,
			TargetType: TargetTypePermissionLetter,
			TargetID:   record.ID,
			Action:     ActionPermissionLetterUploaded,
			Snapshot:   snapshot,
		})
	})
	if err != nil {
		if appErr := asAppError(err); appErr != nil {
			return nil, appErr
		}
		return nil, utils.ErrInternal
	}

	full, err := s.attendances.FindPermissionLetterByID(record.ID)
	if err != nil {
		return nil, utils.ErrInternal
	}
	resp := toPermissionLetterResponse(full)
	return &resp, nil
}

// ListPermissionLetters mengambil daftar surat izin pulang/dispensasi dengan scoping role (F5.8).
func (s *StudentAttendanceService) ListPermissionLetters(ctx context.Context, actorID int64, actorRoles []string, filter dto.PermissionLetterListFilter) (*dto.PermissionLetterListResponse, error) {
	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	dateFrom, dateTo, err := s.resolveOptionalDateFilter("", filter.DateFrom, filter.DateTo)
	if err != nil {
		return nil, err
	}

	tp := strings.ToLower(strings.TrimSpace(filter.Type))
	if tp != "" && tp != models.PermissionLetterTypePulangTatib && tp != models.PermissionLetterTypeDispensasi {
		return nil, validationError("type", utils.FieldInvalidStatus, "type harus pulang_tatib atau dispensasi")
	}

	params := repositories.PermissionLetterListParams{
		Page:      page,
		Limit:     limit,
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		ClassID:   filter.ClassID,
		StudentID: filter.StudentID,
		Type:      tp,
	}

	if hasAnyRole(actorRoles, models.RoleAdmin, models.RoleWakaKurikulum, models.RoleWakaKesiswaan, models.RoleGuruMapel) {
		// Dapat melihat surat izin sesuai filter.
	} else if hasAnyRole(actorRoles, models.RoleWaliKelas) {
		waliClassIDs, err := s.classes.FindIDsByWaliKelas(actorID)
		if err != nil {
			return nil, utils.ErrInternal
		}
		if filter.ClassID != nil && !containsInt64(waliClassIDs, *filter.ClassID) {
			return nil, utils.ErrForbidden
		}
		params.RestrictToClassIDs = waliClassIDs
	} else if hasAnyRole(actorRoles, models.RoleSiswaInti) {
		student, err := s.students.FindByUserID(actorID)
		if err != nil || student.ClassID == nil {
			params.DenyAll = true
		} else {
			if filter.ClassID != nil && *filter.ClassID != *student.ClassID {
				return nil, utils.ErrForbidden
			}
			params.RestrictToClassIDs = []int64{*student.ClassID}
		}
	} else if hasAnyRole(actorRoles, models.RoleSiswa) {
		student, err := s.students.FindByUserID(actorID)
		if err != nil {
			params.DenyAll = true
		} else {
			if filter.StudentID != nil && *filter.StudentID != student.ID {
				return nil, utils.ErrForbidden
			}
			sid := student.ID
			params.RestrictToStudent = &sid
		}
	} else {
		return nil, utils.ErrForbidden
	}

	items, total, err := s.attendances.ListPermissionLetters(params)
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.PermissionLetterResponse, 0, len(items))
	for i := range items {
		respItems = append(respItems, toPermissionLetterResponse(&items[i]))
	}

	return &dto.PermissionLetterListResponse{
		Items: respItems,
		Meta:  buildPaginationMeta(page, limit, total),
	}, nil
}

// =============================================================================
// F5.10 — Akumulasi Alpha & Peringatan Dini (3 / 5 / 9 Hari)
// =============================================================================

// ListAlphaWarnings mengambil daftar log peringatan dini Alpha (3, 5, 9 hari) (F5.10).
func (s *StudentAttendanceService) ListAlphaWarnings(ctx context.Context, actorID int64, actorRoles []string, filter dto.AlphaWarningListFilter) (*dto.AlphaWarningListResponse, error) {
	page, limit, err := normalizePageLimit(filter.Page, filter.Limit)
	if err != nil {
		return nil, err
	}

	if filter.Threshold != nil && *filter.Threshold != 3 && *filter.Threshold != 5 && *filter.Threshold != 9 {
		return nil, validationError("threshold", utils.FieldInvalidStatus, "threshold harus 3, 5, atau 9")
	}

	params := repositories.AlphaWarningListParams{
		Page:           page,
		Limit:          limit,
		AcademicYearID: filter.AcademicYearID,
		ClassID:        filter.ClassID,
		StudentID:      filter.StudentID,
		Threshold:      filter.Threshold,
	}

	if hasAnyRole(actorRoles, models.RoleAdmin, models.RoleWakaKurikulum, models.RoleWakaKesiswaan) {
		// Seluruh sekolah.
	} else if hasAnyRole(actorRoles, models.RoleWaliKelas) {
		waliClassIDs, err := s.classes.FindIDsByWaliKelas(actorID)
		if err != nil {
			return nil, utils.ErrInternal
		}
		if filter.ClassID != nil && !containsInt64(waliClassIDs, *filter.ClassID) {
			return nil, utils.ErrForbidden
		}
		params.RestrictToClassIDs = waliClassIDs
	} else if hasAnyRole(actorRoles, models.RoleSiswa, models.RoleSiswaInti) {
		student, err := s.students.FindByUserID(actorID)
		if err != nil {
			params.DenyAll = true
		} else {
			if filter.StudentID != nil && *filter.StudentID != student.ID {
				return nil, utils.ErrForbidden
			}
			sid := student.ID
			params.RestrictToStudent = &sid
		}
	} else {
		return nil, utils.ErrForbidden
	}

	items, total, err := s.attendances.ListAlphaWarnings(params)
	if err != nil {
		return nil, utils.ErrInternal
	}

	respItems := make([]dto.AlphaWarningItem, 0, len(items))
	for i := range items {
		var stSummary *dto.StudentSummary
		if items[i].Student.ID > 0 {
			stSummary = &dto.StudentSummary{
				ID:       items[i].Student.ID,
				UserID:   items[i].Student.UserID,
				Name:     items[i].Student.User.Name,
				NISN:     items[i].Student.NISN,
				ClassID:  items[i].Student.ClassID,
				IsActive: items[i].Student.User.IsActive,
			}
		}
		respItems = append(respItems, dto.AlphaWarningItem{
			ID:             items[i].ID,
			StudentID:      items[i].StudentID,
			Student:        stSummary,
			AcademicYearID: items[i].AcademicYearID,
			Threshold:      items[i].Threshold,
			AlphaDays:      items[i].AlphaDays,
			TriggeredAt:    items[i].TriggeredAt,
		})
	}

	return &dto.AlphaWarningListResponse{
		Items: respItems,
		Meta:  buildPaginationMeta(page, limit, total),
	}, nil
}

// recomputeStudentAlphaOnDateTx menghitung ulang status Alpha harian siswa (>75% jam
// pelajaran terjadwal pada hari tersebut berstatus alpha murni) dan memicu log
// ALPHA_WARNING_TRIGGERED ketika total hari Alpha dalam 1 tahun ajaran melewati
// ambang 3, 5, atau 9 hari (F5.10).
func (s *StudentAttendanceService) recomputeStudentAlphaOnDateTx(
	ctx context.Context,
	tx *gorm.DB,
	studentID, classID, academicYearID int64,
	date time.Time,
	now time.Time,
) error {
	txAtt := s.attendances.WithTx(tx)

	beforeAlphaDays, err := txAtt.CountStudentAlphaDaysInAcademicYear(studentID, academicYearID)
	if err != nil {
		return err
	}

	lessonTotal, err := txAtt.CountClassLessonSchedulesOnDate(classID, date)
	if err != nil {
		return err
	}

	lessonAlpha, err := txAtt.CountStudentAlphaLessonsOnDate(studentID, classID, date)
	if err != nil {
		return err
	}

	if lessonAlpha > lessonTotal {
		lessonAlpha = lessonTotal
	}

	// Aturan PRD F5.10 & 023_alpha_tracking.sql:
	// is_alpha = TRUE bila > 75% jam pelajaran hari itu berstatus alpha murni (lesson_alpha * 4 > lesson_total * 3).
	isAlpha := lessonTotal > 0 && (int(lessonAlpha)*4 > int(lessonTotal)*3)

	summary := &models.AlphaDailySummary{
		StudentID:      studentID,
		ClassID:        classID,
		AcademicYearID: academicYearID,
		Date:           date,
		LessonTotal:    lessonTotal,
		LessonAlpha:    lessonAlpha,
		IsAlpha:        isAlpha,
		UpdatedAt:      now,
	}
	if err := txAtt.UpsertAlphaDailySummary(summary); err != nil {
		return err
	}

	afterAlphaDays, err := txAtt.CountStudentAlphaDaysInAcademicYear(studentID, academicYearID)
	if err != nil {
		return err
	}

	// Jika melintasi ambang 3, 5, atau 9 hari dari bawah ke atas, catat ke alpha_warning_logs + activity_logs.
	for _, threshold := range alphaWarningThresholds {
		if beforeAlphaDays < threshold && afterAlphaDays >= threshold {
			warnRec := &models.AlphaWarningLog{
				StudentID:      studentID,
				AcademicYearID: academicYearID,
				Threshold:      threshold,
				AlphaDays:      afterAlphaDays,
				TriggeredAt:    now,
			}
			if err := txAtt.CreateAlphaWarningLog(warnRec); err != nil {
				return err
			}

			snapshot, err := marshalAuditSnapshot(map[string]interface{}{
				"warning_id":        warnRec.ID,
				"student_id":        studentID,
				"class_id":          classID,
				"academic_year_id":  academicYearID,
				"threshold":         threshold,
				"alpha_days_before": beforeAlphaDays,
				"alpha_days_after":  afterAlphaDays,
				"triggered_on_date": date.Format(dateFormatYYYYMMDD),
			})
			if err != nil {
				return err
			}
			if err := s.audit.LogTx(ctx, tx, AuditEvent{
				ActorID:    0, // NULL = proses otomatis sistem (F5.10)
				TargetType: TargetTypeAlphaWarning,
				TargetID:   warnRec.ID,
				Action:     ActionAlphaWarningTriggered,
				Snapshot:   snapshot,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// =============================================================================
// Scoping & Conversion Helpers
// =============================================================================

func (s *StudentAttendanceService) applyStudentAttendanceScoping(actorID int64, actorRoles []string, params *repositories.StudentAttendanceListParams) error {
	if hasAnyRole(actorRoles, models.RoleAdmin, models.RoleWakaKurikulum, models.RoleWakaKesiswaan) {
		return nil
	}

	// Siswa & Siswa Inti hanya boleh melihat absensi dirinya sendiri pada GET /attendances & GET /attendances/recap (F5.4).
	if hasAnyRole(actorRoles, models.RoleSiswa, models.RoleSiswaInti) {
		student, err := s.students.FindByUserID(actorID)
		if err != nil {
			params.DenyAll = true
			return nil
		}
		if params.StudentID != nil && *params.StudentID != student.ID {
			return utils.ErrForbidden.WithMessage("Siswa hanya dapat melihat data absensi miliknya sendiri")
		}
		if params.ClassID != nil && (student.ClassID == nil || *params.ClassID != *student.ClassID) {
			return utils.ErrForbidden.WithMessage("Anda tidak memiliki akses ke kelas lain")
		}
		sid := student.ID
		params.RestrictToStudent = &sid
		return nil
	}

	hasWali := hasAnyRole(actorRoles, models.RoleWaliKelas)
	hasGuru := hasAnyRole(actorRoles, models.RoleGuruMapel)

	var waliClassIDs []int64
	if hasWali {
		ids, err := s.classes.FindIDsByWaliKelas(actorID)
		if err != nil {
			return utils.ErrInternal
		}
		waliClassIDs = ids
	}

	var myTeacherID *int64
	if hasGuru {
		teacher, err := s.teachers.FindByUserID(actorID)
		if err == nil {
			tid := teacher.ID
			myTeacherID = &tid
		}
	}

	switch {
	case hasWali && !hasGuru:
		if params.ClassID != nil && !containsInt64(waliClassIDs, *params.ClassID) {
			return utils.ErrForbidden.WithMessage("Wali Kelas hanya dapat mengakses data absensi kelas perwaliannya")
		}
		if len(waliClassIDs) == 0 {
			params.DenyAll = true
			return nil
		}
		params.RestrictToClassIDs = waliClassIDs
		return nil

	case hasGuru && !hasWali:
		if myTeacherID == nil {
			params.DenyAll = true
			return nil
		}
		params.RestrictToTeacher = myTeacherID
		return nil

	case hasWali && hasGuru:
		if params.ClassID != nil && !containsInt64(waliClassIDs, *params.ClassID) {
			if myTeacherID == nil {
				return utils.ErrForbidden
			}
			params.RestrictToTeacher = myTeacherID
			return nil
		}
		if len(waliClassIDs) == 0 && myTeacherID == nil {
			params.DenyAll = true
			return nil
		}
		params.RestrictToClassIDs = waliClassIDs
		params.RestrictToTeacher = myTeacherID
		return nil

	default:
		return utils.ErrForbidden
	}
}

func (s *StudentAttendanceService) applyLeaveRequestScoping(actorID int64, actorRoles []string, params *repositories.LeaveRequestListParams) (bool, *int64, error) {
	if hasAnyRole(actorRoles, models.RoleAdmin, models.RoleWakaKurikulum, models.RoleWakaKesiswaan) {
		return false, nil, nil
	}

	if hasAnyRole(actorRoles, models.RoleWaliKelas) {
		waliClassIDs, err := s.classes.FindIDsByWaliKelas(actorID)
		if err != nil {
			return false, nil, utils.ErrInternal
		}
		if params.ClassID != nil && !containsInt64(waliClassIDs, *params.ClassID) {
			return false, nil, utils.ErrForbidden
		}
		params.RestrictToClassIDs = waliClassIDs
		return false, nil, nil
	}

	if hasAnyRole(actorRoles, models.RoleGuruMapel) {
		// Guru Mapel boleh melihat status izin/sakit siswa namun alasan & bukti pribadi disamarkan (F5.9).
		return true, nil, nil
	}

	if hasAnyRole(actorRoles, models.RoleSiswaInti) {
		student, err := s.students.FindByUserID(actorID)
		if err != nil || student.ClassID == nil {
			params.DenyAll = true
			return true, nil, nil
		}
		if params.ClassID != nil && *params.ClassID != *student.ClassID {
			return true, nil, utils.ErrForbidden
		}
		params.RestrictToClassIDs = []int64{*student.ClassID}
		sid := student.ID
		// Siswa Inti melihat detail penuh untuk pengajuannya sendiri, dan ter-masking untuk teman sekelasnya (F5.9).
		return true, &sid, nil
	}

	if hasAnyRole(actorRoles, models.RoleSiswa) {
		student, err := s.students.FindByUserID(actorID)
		if err != nil {
			params.DenyAll = true
			return false, nil, nil
		}
		if params.StudentID != nil && *params.StudentID != student.ID {
			return false, nil, utils.ErrForbidden
		}
		sid := student.ID
		params.RestrictToStudent = &sid
		return false, &sid, nil
	}

	return true, nil, utils.ErrForbidden
}

func (s *StudentAttendanceService) resolveStudentAttendanceDateRange(dateStr, fromStr, toStr, preset string) (time.Time, time.Time, error) {
	if strings.TrimSpace(dateStr) != "" {
		d, err := parseDateField("date", dateStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		return d, d, nil
	}

	now := s.now().In(jakartaLocation())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, jakartaLocation())

	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "today":
		return today, today, nil
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
	if fromStr == "" && toStr == "" {
		firstOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, jakartaLocation())
		lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
		return firstOfMonth, lastOfMonth, nil
	}

	var (
		fromDate time.Time
		toDate   time.Time
		err      error
	)
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

func (s *StudentAttendanceService) resolveOptionalDateFilter(dateStr, fromStr, toStr string) (time.Time, time.Time, error) {
	if strings.TrimSpace(dateStr) != "" {
		d, err := parseDateField("date", dateStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		return d, d, nil
	}
	var (
		fromDate time.Time
		toDate   time.Time
		err      error
	)
	if strings.TrimSpace(fromStr) != "" {
		fromDate, err = parseDateField("date_from", fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if strings.TrimSpace(toStr) != "" {
		toDate, err = parseDateField("date_to", toStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if !fromDate.IsZero() && !toDate.IsZero() && fromDate.After(toDate) {
		return time.Time{}, time.Time{}, validationError("date_to", utils.FieldInvalidDateRange, "date_from tidak boleh lebih besar dari date_to")
	}
	return fromDate, toDate, nil
}

func isValidStudentAttendanceStatus(st string) bool {
	switch st {
	case models.StudentAttendanceStatusHadir,
		models.StudentAttendanceStatusIzin,
		models.StudentAttendanceStatusSakit,
		models.StudentAttendanceStatusAlpha,
		models.StudentAttendanceStatusDispensasi:
		return true
	default:
		return false
	}
}

func toStudentAttendanceResponse(rec *models.StudentAttendance) dto.StudentAttendanceResponse {
	resp := dto.StudentAttendanceResponse{
		ID:             rec.ID,
		ScheduleID:     rec.ScheduleID,
		StudentID:      rec.StudentID,
		ClassID:        rec.ClassID,
		AcademicYearID: rec.AcademicYearID,
		Status:         rec.Status,
		Source:         rec.Source,
		RecordedBy:     rec.RecordedBy,
		Note:           rec.Note,
		Version:        rec.Version,
		UpdatedAt:      rec.UpdatedAt,
	}
	if rec.Schedule.ID > 0 {
		schedResp := toScheduleResponse(&rec.Schedule)
		resp.Schedule = &schedResp
	}
	if rec.Student.ID > 0 {
		resp.Student = &dto.StudentSummary{
			ID:       rec.Student.ID,
			UserID:   rec.Student.UserID,
			Name:     rec.Student.User.Name,
			NISN:     rec.Student.NISN,
			ClassID:  rec.Student.ClassID,
			IsActive: rec.Student.User.IsActive,
		}
	}
	if rec.Class.ID > 0 {
		resp.Class = &dto.ScheduleClassSummary{
			ID:         rec.Class.ID,
			Name:       rec.Class.Name,
			GradeLevel: rec.Class.GradeLevel,
			IsActive:   rec.Class.IsActive,
		}
	}
	return resp
}

func toLeaveRequestResponse(rec *models.LeaveRequest, maskSensitive bool) dto.LeaveRequestResponse {
	resp := dto.LeaveRequestResponse{
		ID:         rec.ID,
		StudentID:  rec.StudentID,
		ClassID:    rec.ClassID,
		Type:       rec.Type,
		Date:       rec.Date.Format(dateFormatYYYYMMDD),
		Status:     rec.Status,
		ReviewedBy: rec.ReviewedBy,
		ReviewedAt: rec.ReviewedAt,
		CreatedAt:  rec.CreatedAt,
	}
	if !maskSensitive {
		resp.Reason = rec.Reason
		resp.EvidenceFileURL = rec.EvidenceFileURL
	}
	if rec.Student.ID > 0 {
		resp.Student = &dto.StudentSummary{
			ID:       rec.Student.ID,
			UserID:   rec.Student.UserID,
			Name:     rec.Student.User.Name,
			NISN:     rec.Student.NISN,
			ClassID:  rec.Student.ClassID,
			IsActive: rec.Student.User.IsActive,
		}
	}
	if rec.Class.ID > 0 {
		resp.Class = &dto.ScheduleClassSummary{
			ID:         rec.Class.ID,
			Name:       rec.Class.Name,
			GradeLevel: rec.Class.GradeLevel,
			IsActive:   rec.Class.IsActive,
		}
	}
	return resp
}

func toPermissionLetterResponse(rec *models.PermissionLetter) dto.PermissionLetterResponse {
	resp := dto.PermissionLetterResponse{
		ID:              rec.ID,
		StudentID:       rec.StudentID,
		Type:            rec.Type,
		UploadedBy:      rec.UploadedBy,
		EvidenceFileURL: rec.EvidenceFileURL,
		Note:            rec.Note,
		CreatedAt:       rec.CreatedAt,
	}
	if rec.Student.ID > 0 {
		resp.Student = &dto.StudentSummary{
			ID:       rec.Student.ID,
			UserID:   rec.Student.UserID,
			Name:     rec.Student.User.Name,
			NISN:     rec.Student.NISN,
			ClassID:  rec.Student.ClassID,
			IsActive: rec.Student.User.IsActive,
		}
	}
	if rec.Uploader.ID > 0 {
		resp.UploaderName = rec.Uploader.Name
	}
	return resp
}
