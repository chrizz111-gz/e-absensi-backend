package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/dto"
	"e-absensi-backend/middlewares"
	"e-absensi-backend/services"
	"e-absensi-backend/utils"
)

// ClassController menangani endpoint F3.3 Kelas.
type ClassController struct {
	classes *services.ClassService
}

// NewClassController membuat ClassController baru.
func NewClassController(classes *services.ClassService) *ClassController {
	return &ClassController{classes: classes}
}

// List menangani GET /api/v1/classes.
func (h *ClassController) List(c *gin.Context) {
	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	isActive, ok := parseOptionalActiveFilter(c)
	if !ok {
		return
	}

	academicYearID, ok := parseOptionalInt64Query(c, "academic_year_id")
	if !ok {
		return
	}

	var gradeLevel *int16
	if rawGrade := strings.TrimSpace(c.Query("grade_level")); rawGrade != "" {
		parsed, err := strconv.ParseInt(rawGrade, 10, 16)
		if err != nil || (parsed != 10 && parsed != 11 && parsed != 12) {
			c.Error(utils.NewValidationError([]utils.FieldError{{
				Field:   "grade_level",
				Code:    utils.FieldInvalidGradeLevel,
				Message: "grade_level harus 10, 11, atau 12",
			}}))
			return
		}
		g := int16(parsed)
		gradeLevel = &g
	}

	result, err := h.classes.List(c.Request.Context(), dto.ClassListFilter{
		Page:           page,
		Limit:          limit,
		Q:              parseSearchQuery(c),
		AcademicYearID: academicYearID,
		GradeLevel:     gradeLevel,
		IsActive:       isActive,
	})
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar kelas berhasil diambil", result)
}

// Get menangani GET /api/v1/classes/{id}.
func (h *ClassController) Get(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	roles, _ := middlewares.CurrentRoles(c)

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	result, err := h.classes.Get(c.Request.Context(), actorID, roles, id)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Detail kelas berhasil diambil", result)
}

// Create menangani POST /api/v1/classes.
func (h *ClassController) Create(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}

	var req dto.CreateClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.classes.Create(c.Request.Context(), actorID, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, utils.CodeCreated, "Kelas berhasil dibuat", result)
}

// Update menangani PATCH /api/v1/classes/{id}.
func (h *ClassController) Update(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.UpdateClassRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.classes.Update(c.Request.Context(), actorID, id, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Kelas berhasil diperbarui", result)
}

// Delete menangani DELETE /api/v1/classes/{id}.
func (h *ClassController) Delete(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	if err := h.classes.Delete(c.Request.Context(), actorID, id); err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Kelas berhasil dihapus", nil)
}

// parseOptionalInt64Query membaca query integer positif opsional.
func parseOptionalInt64Query(c *gin.Context, key string) (*int64, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, true
	}
	val, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || val < 1 {
		c.Error(utils.NewValidationError([]utils.FieldError{{
			Field:   key,
			Code:    utils.FieldInvalidID,
			Message: key + " tidak valid",
		}}))
		return nil, false
	}
	return &val, true
}
