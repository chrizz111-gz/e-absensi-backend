package controllers

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/dto"
	"e-absensi-backend/middlewares"
	"e-absensi-backend/services"
	"e-absensi-backend/utils"
)

// TeacherAttendanceController menangani endpoint HTTP EPIC 6 (Teacher Attendance & QR).
type TeacherAttendanceController struct {
	service *services.TeacherAttendanceService
}

// NewTeacherAttendanceController membuat instance TeacherAttendanceController.
func NewTeacherAttendanceController(service *services.TeacherAttendanceService) *TeacherAttendanceController {
	return &TeacherAttendanceController{service: service}
}

// GenerateQR menangani POST /api/v1/teacher-attendance/qr/generate (F6.1).
func (ctrl *TeacherAttendanceController) GenerateQR(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	var req dto.GenerateTeacherQRRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
			c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
			return
		}
	}

	resp, err := ctrl.service.GenerateQR(c.Request.Context(), actorID, actorRoles, req)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "QR Code dan kode cadangan kehadiran guru berhasil dibuat", resp)
}

// Confirm menangani POST /api/v1/teacher-attendance/confirm (F6.2).
func (ctrl *TeacherAttendanceController) Confirm(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}

	var req dto.ConfirmTeacherAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	resp, err := ctrl.service.Confirm(c.Request.Context(), actorID, req)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Kehadiran guru berhasil dikonfirmasi", resp)
}

// List menangani GET /api/v1/teacher-attendance (F6.4).
func (ctrl *TeacherAttendanceController) List(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	teacherID, ok := parseOptionalInt64Query(c, "teacher_id")
	if !ok {
		return
	}
	classID, ok := parseOptionalInt64Query(c, "class_id")
	if !ok {
		return
	}

	filter := dto.TeacherAttendanceListFilter{
		Page:      page,
		Limit:     limit,
		DateFrom:  strings.TrimSpace(c.Query("date_from")),
		DateTo:    strings.TrimSpace(c.Query("date_to")),
		Preset:    strings.TrimSpace(c.Query("preset")),
		TeacherID: teacherID,
		ClassID:   classID,
		Status:    strings.TrimSpace(c.Query("status")),
	}

	resp, err := ctrl.service.List(c.Request.Context(), actorID, actorRoles, filter)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Rekap kehadiran guru berhasil diambil", resp)
}

// Get menangani GET /api/v1/teacher-attendance/:id (F6.4).
func (ctrl *TeacherAttendanceController) Get(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	resp, err := ctrl.service.Get(c.Request.Context(), actorID, actorRoles, id)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Detail kehadiran guru berhasil diambil", resp)
}

// Annotate menangani PATCH /api/v1/teacher-attendance/:id/keterangan (F6.4).
func (ctrl *TeacherAttendanceController) Annotate(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.AnnotateTeacherAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	resp, err := ctrl.service.Annotate(c.Request.Context(), actorID, id, req)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Keterangan kehadiran guru berhasil diperbarui", resp)
}

// CorrectStatus menangani PATCH /api/v1/teacher-attendance/:id/correct (F6.4).
func (ctrl *TeacherAttendanceController) CorrectStatus(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.CorrectTeacherAttendanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	resp, err := ctrl.service.CorrectStatus(c.Request.Context(), actorID, id, req)
	if err != nil {
		c.Error(err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Status kehadiran guru berhasil dikoreksi", resp)
}

// Export menangani GET /api/v1/teacher-attendance/export (F6.5).
func (ctrl *TeacherAttendanceController) Export(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	actorRoles, _ := middlewares.CurrentRoles(c)

	teacherID, ok := parseOptionalInt64Query(c, "teacher_id")
	if !ok {
		return
	}
	classID, ok := parseOptionalInt64Query(c, "class_id")
	if !ok {
		return
	}

	filter := dto.TeacherAttendanceListFilter{
		DateFrom:  strings.TrimSpace(c.Query("date_from")),
		DateTo:    strings.TrimSpace(c.Query("date_to")),
		Preset:    strings.TrimSpace(c.Query("preset")),
		TeacherID: teacherID,
		ClassID:   classID,
		Status:    strings.TrimSpace(c.Query("status")),
	}

	content, filename, err := ctrl.service.Export(c.Request.Context(), actorID, actorRoles, filter)
	if err != nil {
		c.Error(err)
		return
	}

	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}
