package controllers

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/dto"
	"e-absensi-backend/middlewares"
	"e-absensi-backend/services"
	"e-absensi-backend/utils"
)

// ScheduleController menangani endpoint Manajemen Jadwal (Epic 4 F4.1-F4.6).
type ScheduleController struct {
	schedules *services.ScheduleService
}

// NewScheduleController membuat ScheduleController baru.
func NewScheduleController(schedules *services.ScheduleService) *ScheduleController {
	return &ScheduleController{schedules: schedules}
}

// DownloadTemplate menangani GET /api/v1/schedules/template (F4.1 - khusus Admin).
func (h *ScheduleController) DownloadTemplate(c *gin.Context) {
	data, err := h.schedules.GenerateTemplate(c.Request.Context())
	if err != nil {
		c.Error(utils.ErrInternal)
		return
	}

	c.Header("Content-Disposition", `attachment; filename="template_jadwal.xlsx"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
}

// PreviewImport menangani POST /api/v1/schedules/import/preview (F4.2 - khusus Admin).
func (h *ScheduleController) PreviewImport(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		c.Error(utils.ErrImportFileInvalid.WithField("file", utils.FieldRequired, "File Excel (.xlsx) wajib diunggah pada field 'file'"))
		return
	}

	if ext := strings.ToLower(filepath.Ext(fileHeader.Filename)); ext != ".xlsx" {
		c.Error(utils.ErrImportFileInvalid.WithField("file", utils.CodeImportFileInvalid, "File harus berformat .xlsx"))
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.Error(utils.ErrImportFileInvalid)
		return
	}
	defer f.Close()

	fileBytes, err := io.ReadAll(io.LimitReader(f, 10*1024*1024))
	if err != nil || len(fileBytes) == 0 {
		c.Error(utils.ErrImportFileInvalid)
		return
	}

	result, err := h.schedules.PreviewImport(c.Request.Context(), actorID, fileBytes)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Validasi preview import jadwal selesai", result)
}

// ConfirmImport menangani POST /api/v1/schedules/import/confirm (F4.2 - khusus Admin).
func (h *ScheduleController) ConfirmImport(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}

	var req dto.ScheduleImportConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.ErrImportNotValidated)
		return
	}

	result, err := h.schedules.ConfirmImport(c.Request.Context(), actorID, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, utils.CodeCreated, "Jadwal berhasil diimport", result)
}

// List menangani GET /api/v1/schedules (F4.3).
func (h *ScheduleController) List(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	roles, _ := middlewares.CurrentRoles(c)

	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	classID, ok := parseOptionalInt64Query(c, "class_id")
	if !ok {
		return
	}
	teacherID, ok := parseOptionalInt64Query(c, "teacher_id")
	if !ok {
		return
	}
	subjectID, ok := parseOptionalInt64Query(c, "subject_id")
	if !ok {
		return
	}

	result, err := h.schedules.List(c.Request.Context(), actorID, roles, dto.ScheduleListFilter{
		Page:      page,
		Limit:     limit,
		DateFrom:  strings.TrimSpace(c.Query("date_from")),
		DateTo:    strings.TrimSpace(c.Query("date_to")),
		ClassID:   classID,
		TeacherID: teacherID,
		SubjectID: subjectID,
		Type:      strings.TrimSpace(c.Query("type")),
	})
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar jadwal berhasil diambil", result)
}

// Get menangani GET /api/v1/schedules/{id} (F4.3).
func (h *ScheduleController) Get(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	roles, _ := middlewares.CurrentRoles(c)

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	result, err := h.schedules.Get(c.Request.Context(), actorID, roles, id)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Detail jadwal berhasil diambil", result)
}

// Update menangani PATCH /api/v1/schedules/{id} (F4.4).
func (h *ScheduleController) Update(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.UpdateScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.schedules.Update(c.Request.Context(), actorID, id, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Jadwal berhasil diperbarui", result)
}

// ConvertToActivity menangani PATCH /api/v1/schedules/{id}/convert-to-activity (F4.6).
func (h *ScheduleController) ConvertToActivity(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.ConvertScheduleToActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.schedules.ConvertToActivity(c.Request.Context(), actorID, id, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Jadwal berhasil dikonversi menjadi kegiatan", result)
}

// RevertToLesson menangani PATCH /api/v1/schedules/{id}/revert-to-lesson (F4.6).
func (h *ScheduleController) RevertToLesson(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.RevertScheduleToLessonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.schedules.RevertToLesson(c.Request.Context(), actorID, id, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Jadwal berhasil dikembalikan menjadi pelajaran", result)
}

// Delete menangani DELETE /api/v1/schedules/{id} (F4.5 - khusus Admin).
func (h *ScheduleController) Delete(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	if err := h.schedules.Delete(c.Request.Context(), actorID, id); err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Jadwal berhasil dihapus", nil)
}
