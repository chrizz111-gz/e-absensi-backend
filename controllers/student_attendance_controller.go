package controllers

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"e-absensi-backend/dto"
	"e-absensi-backend/middlewares"
	"e-absensi-backend/services"
	"e-absensi-backend/utils"
)

// maxEvidenceFileSize adalah ukuran maksimum file bukti izin/sakit/surat (5 MB sesuai F5.6 & F5.8).
const maxEvidenceFileSize = 5 * 1024 * 1024

// StudentAttendanceController menangani seluruh endpoint HTTP EPIC 5 (Student Attendance F5.1-F5.10).
type StudentAttendanceController struct {
	service *services.StudentAttendanceService
}

// NewStudentAttendanceController membuat instance StudentAttendanceController.
func NewStudentAttendanceController(service *services.StudentAttendanceService) *StudentAttendanceController {
	return &StudentAttendanceController{service: service}
}

// GetAttendanceForm menangani GET /api/v1/schedules/:id/attendance-form (F5.1 & F5.2).
func (ctrl *StudentAttendanceController) GetAttendanceForm(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	scheduleID, ok := parseUserID(c)
	if !ok {
		return
	}

	resp, err := ctrl.service.GetAttendanceForm(c.Request.Context(), actorID, actorRoles, scheduleID)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Form ceklis absensi siswa berhasil dimuat", resp)
}

// SubmitScheduleAttendances menangani POST /api/v1/schedules/:id/attendances (F5.1 & F5.2).
func (ctrl *StudentAttendanceController) SubmitScheduleAttendances(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	scheduleID, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.SubmitScheduleAttendancesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	resp, err := ctrl.service.SubmitScheduleAttendances(c.Request.Context(), actorID, actorRoles, scheduleID, req)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, utils.CodeAttendanceRecorded, "Absensi siswa berhasil disimpan", resp)
}

// UpdateAttendance menangani PATCH /api/v1/attendances/:id (F5.3).
func (ctrl *StudentAttendanceController) UpdateAttendance(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.UpdateStudentAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	resp, err := ctrl.service.UpdateAttendance(c.Request.Context(), actorID, actorRoles, id, req)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Status absensi siswa berhasil diperbarui", resp)
}

// ListAttendances menangani GET /api/v1/attendances (F5.4).
func (ctrl *StudentAttendanceController) ListAttendances(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	classID, ok := parseOptionalInt64Query(c, "class_id")
	if !ok {
		return
	}
	scheduleID, ok := parseOptionalInt64Query(c, "schedule_id")
	if !ok {
		return
	}
	studentID, ok := parseOptionalInt64Query(c, "student_id")
	if !ok {
		return
	}
	subjectID, ok := parseOptionalInt64Query(c, "subject_id")
	if !ok {
		return
	}

	filter := dto.StudentAttendanceListFilter{
		Page:       page,
		Limit:      limit,
		Date:       strings.TrimSpace(c.Query("date")),
		DateFrom:   strings.TrimSpace(c.Query("date_from")),
		DateTo:     strings.TrimSpace(c.Query("date_to")),
		Preset:     strings.TrimSpace(c.Query("preset")),
		ClassID:    classID,
		ScheduleID: scheduleID,
		StudentID:  studentID,
		SubjectID:  subjectID,
		Status:     strings.TrimSpace(c.Query("status")),
		Source:     strings.TrimSpace(c.Query("source")),
	}

	resp, err := ctrl.service.ListAttendances(c.Request.Context(), actorID, actorRoles, filter)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar absensi siswa berhasil diambil", resp)
}

// RecapAttendances menangani GET /api/v1/attendances/recap (F5.5).
func (ctrl *StudentAttendanceController) RecapAttendances(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	classID, ok := parseOptionalInt64Query(c, "class_id")
	if !ok {
		return
	}
	studentID, ok := parseOptionalInt64Query(c, "student_id")
	if !ok {
		return
	}

	filter := dto.StudentAttendanceListFilter{
		Page:      page,
		Limit:     limit,
		Date:      strings.TrimSpace(c.Query("date")),
		DateFrom:  strings.TrimSpace(c.Query("date_from")),
		DateTo:    strings.TrimSpace(c.Query("date_to")),
		Preset:    strings.TrimSpace(c.Query("preset")),
		ClassID:   classID,
		StudentID: studentID,
	}

	resp, err := ctrl.service.RecapAttendances(c.Request.Context(), actorID, actorRoles, filter)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Rekap absensi siswa berhasil diambil", resp)
}

// ExportAttendances menangani GET /api/v1/attendances/export (F5.5).
func (ctrl *StudentAttendanceController) ExportAttendances(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	classID, ok := parseOptionalInt64Query(c, "class_id")
	if !ok {
		return
	}
	studentID, ok := parseOptionalInt64Query(c, "student_id")
	if !ok {
		return
	}

	filter := dto.StudentAttendanceListFilter{
		Date:      strings.TrimSpace(c.Query("date")),
		DateFrom:  strings.TrimSpace(c.Query("date_from")),
		DateTo:    strings.TrimSpace(c.Query("date_to")),
		Preset:    strings.TrimSpace(c.Query("preset")),
		ClassID:   classID,
		StudentID: studentID,
	}

	content, filename, err := ctrl.service.ExportAttendances(c.Request.Context(), actorID, actorRoles, filter)
	if err != nil {
		c.Error(err)
		return
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}

// SubmitLeaveRequest menangani POST /api/v1/leave-requests (F5.6).
// Mendukung multipart/form-data (dengan upload file evidence_file <= 5MB) maupun JSON.
func (ctrl *StudentAttendanceController) SubmitLeaveRequest(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}

	var payload dto.SubmitLeaveRequestPayload
	contentType := strings.ToLower(c.GetHeader("Content-Type"))

	if strings.Contains(contentType, "multipart/form-data") {
		payload.Type = strings.TrimSpace(c.PostForm("type"))
		payload.Date = strings.TrimSpace(c.PostForm("date"))
		payload.Reason = strings.TrimSpace(c.PostForm("reason"))
		payload.EvidenceFileURL = strings.TrimSpace(c.PostForm("evidence_file_url"))

		fileHeader, err := c.FormFile("evidence_file")
		if err != nil {
			fileHeader, err = c.FormFile("file")
		}
		if err == nil && fileHeader != nil {
			savedURL, uploadErr := saveUploadedEvidenceFile(fileHeader, "leave_requests")
			if uploadErr != nil {
				c.Error(uploadErr)
				return
			}
			payload.EvidenceFileURL = savedURL
		}
	} else {
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
			return
		}
	}

	resp, err := ctrl.service.SubmitLeaveRequest(c.Request.Context(), actorID, payload)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, utils.CodeCreated, "Pengajuan izin/sakit berhasil dikirim dan menunggu verifikasi Wali Kelas", resp)
}

// ListLeaveRequests menangani GET /api/v1/leave-requests (F5.6, F5.7, F5.9).
func (ctrl *StudentAttendanceController) ListLeaveRequests(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	classID, ok := parseOptionalInt64Query(c, "class_id")
	if !ok {
		return
	}
	studentID, ok := parseOptionalInt64Query(c, "student_id")
	if !ok {
		return
	}

	filter := dto.LeaveRequestListFilter{
		Page:      page,
		Limit:     limit,
		Date:      strings.TrimSpace(c.Query("date")),
		DateFrom:  strings.TrimSpace(c.Query("date_from")),
		DateTo:    strings.TrimSpace(c.Query("date_to")),
		ClassID:   classID,
		StudentID: studentID,
		Type:      strings.TrimSpace(c.Query("type")),
		Status:    strings.TrimSpace(c.Query("status")),
	}

	resp, err := ctrl.service.ListLeaveRequests(c.Request.Context(), actorID, actorRoles, filter)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar pengajuan izin/sakit berhasil diambil", resp)
}

// GetClassTodayLeaveSummary menangani GET /api/v1/leave-requests/class-today (F5.9).
func (ctrl *StudentAttendanceController) GetClassTodayLeaveSummary(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	classID, ok := parseOptionalInt64Query(c, "class_id")
	if !ok {
		return
	}

	resp, err := ctrl.service.GetClassTodayLeaveSummary(c.Request.Context(), actorID, actorRoles, classID, strings.TrimSpace(c.Query("date")))
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Status izin/sakit kelas hari ini berhasil diambil", resp)
}

// ApproveLeaveRequest menangani PATCH /api/v1/leave-requests/:id/approve (F5.7).
func (ctrl *StudentAttendanceController) ApproveLeaveRequest(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	resp, err := ctrl.service.ApproveLeaveRequest(c.Request.Context(), actorID, actorRoles, id)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Pengajuan izin/sakit berhasil disetujui", resp)
}

// RejectLeaveRequest menangani PATCH /api/v1/leave-requests/:id/reject (F5.7).
func (ctrl *StudentAttendanceController) RejectLeaveRequest(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	resp, err := ctrl.service.RejectLeaveRequest(c.Request.Context(), actorID, actorRoles, id)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Pengajuan izin/sakit berhasil ditolak", resp)
}

// UploadPermissionLetter menangani POST /api/v1/permission-letters (F5.8).
func (ctrl *StudentAttendanceController) UploadPermissionLetter(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}

	var payload dto.UploadPermissionLetterPayload
	contentType := strings.ToLower(c.GetHeader("Content-Type"))

	if strings.Contains(contentType, "multipart/form-data") {
		if rawID := strings.TrimSpace(c.PostForm("student_id")); rawID != "" {
			parsedID, err := strconv.ParseInt(rawID, 10, 64)
			if err != nil || parsedID <= 0 {
				c.Error(utils.NewValidationError([]utils.FieldError{{
					Field:   "student_id",
					Code:    utils.FieldInvalidID,
					Message: "student_id harus berupa bilangan bulat positif",
				}}))
				return
			}
			payload.StudentID = parsedID
		}
		payload.Type = strings.TrimSpace(c.PostForm("type"))
		payload.EvidenceFileURL = strings.TrimSpace(c.PostForm("evidence_file_url"))
		if rawNote := strings.TrimSpace(c.PostForm("note")); rawNote != "" {
			payload.Note = &rawNote
		}

		fileHeader, err := c.FormFile("evidence_file")
		if err != nil {
			fileHeader, err = c.FormFile("file")
		}
		if err == nil && fileHeader != nil {
			savedURL, uploadErr := saveUploadedEvidenceFile(fileHeader, "permission_letters")
			if uploadErr != nil {
				c.Error(uploadErr)
				return
			}
			payload.EvidenceFileURL = savedURL
		}
	} else {
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
			return
		}
	}

	resp, err := ctrl.service.UploadPermissionLetter(c.Request.Context(), actorID, payload)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, utils.CodeCreated, "Surat izin berhasil diunggah", resp)
}

// ListPermissionLetters menangani GET /api/v1/permission-letters (F5.8).
func (ctrl *StudentAttendanceController) ListPermissionLetters(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	classID, ok := parseOptionalInt64Query(c, "class_id")
	if !ok {
		return
	}
	studentID, ok := parseOptionalInt64Query(c, "student_id")
	if !ok {
		return
	}

	filter := dto.PermissionLetterListFilter{
		Page:      page,
		Limit:     limit,
		DateFrom:  strings.TrimSpace(c.Query("date_from")),
		DateTo:    strings.TrimSpace(c.Query("date_to")),
		ClassID:   classID,
		StudentID: studentID,
		Type:      strings.TrimSpace(c.Query("type")),
	}

	resp, err := ctrl.service.ListPermissionLetters(c.Request.Context(), actorID, actorRoles, filter)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar surat izin berhasil diambil", resp)
}

// ListAlphaWarnings menangani GET /api/v1/attendances/alpha-warnings (F5.10).
func (ctrl *StudentAttendanceController) ListAlphaWarnings(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	academicYearID, ok := parseOptionalInt64Query(c, "academic_year_id")
	if !ok {
		return
	}
	classID, ok := parseOptionalInt64Query(c, "class_id")
	if !ok {
		return
	}
	studentID, ok := parseOptionalInt64Query(c, "student_id")
	if !ok {
		return
	}

	var thresholdPtr *int16
	if rawThreshold := strings.TrimSpace(c.Query("threshold")); rawThreshold != "" {
		val, err := strconv.ParseInt(rawThreshold, 10, 16)
		if err != nil {
			c.Error(utils.NewValidationError([]utils.FieldError{{
				Field:   "threshold",
				Code:    utils.FieldInvalidStatus,
				Message: "threshold harus berupa angka 3, 5, atau 9",
			}}))
			return
		}
		v16 := int16(val)
		thresholdPtr = &v16
	}

	filter := dto.AlphaWarningListFilter{
		Page:           page,
		Limit:          limit,
		AcademicYearID: academicYearID,
		ClassID:        classID,
		StudentID:      studentID,
		Threshold:      thresholdPtr,
	}

	resp, err := ctrl.service.ListAlphaWarnings(c.Request.Context(), actorID, actorRoles, filter)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar peringatan dini akumulasi Alpha berhasil diambil", resp)
}

func saveUploadedEvidenceFile(fileHeader *multipart.FileHeader, subDir string) (string, error) {
	if fileHeader.Size <= 0 {
		return "", utils.ErrEvidenceFileRequired
	}
	if fileHeader.Size > maxEvidenceFileSize {
		return "", utils.ErrEvidenceFileInvalid.WithField("evidence_file", utils.CodeEvidenceFileInvalid, "Ukuran file bukti maksimal 5 MB")
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".pdf":
	default:
		return "", utils.ErrEvidenceFileInvalid.WithField("evidence_file", utils.CodeEvidenceFileInvalid, "File bukti harus berformat JPG, PNG, WEBP, atau PDF")
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", utils.ErrEvidenceFileInvalid
	}
	defer src.Close()

	data, err := io.ReadAll(io.LimitReader(src, maxEvidenceFileSize+1))
	if err != nil || int64(len(data)) > maxEvidenceFileSize {
		return "", utils.ErrEvidenceFileInvalid
	}

	dirPath := filepath.Join("uploads", subDir)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return "", utils.ErrInternal
	}

	fileName := fmt.Sprintf("%s_%s%s", time.Now().Format("20060102_150405"), uuid.NewString()[:8], ext)
	fullPath := filepath.Join(dirPath, fileName)
	if err := os.WriteFile(fullPath, data, 0o644); err != nil {
		return "", utils.ErrInternal
	}

	return fmt.Sprintf("/uploads/%s/%s", subDir, fileName), nil
}
