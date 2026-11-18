package controllers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/dto"
	"e-absensi-backend/services"
	"e-absensi-backend/utils"
)

// AcademicYearController menangani endpoint F3.5 Tahun Ajaran.
type AcademicYearController struct {
	years *services.AcademicYearService
}

// NewAcademicYearController membuat AcademicYearController baru.
func NewAcademicYearController(years *services.AcademicYearService) *AcademicYearController {
	return &AcademicYearController{years: years}
}

// List menangani GET /api/v1/academic-years.
func (h *AcademicYearController) List(c *gin.Context) {
	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	isActive, ok := parseOptionalActiveFilter(c)
	if !ok {
		return
	}

	result, err := h.years.List(c.Request.Context(), dto.AcademicYearListFilter{
		Page:           page,
		Limit:          limit,
		Q:              strings.TrimSpace(c.Query("q")),
		IsActive:       isActive,
		ActiveSemester: strings.TrimSpace(c.Query("active_semester")),
	})
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar tahun ajaran berhasil diambil", result)
}

// Get menangani GET /api/v1/academic-years/{id}.
func (h *AcademicYearController) Get(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	result, err := h.years.Get(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Detail tahun ajaran berhasil diambil", result)
}

// Create menangani POST /api/v1/academic-years (khusus Admin).
func (h *AcademicYearController) Create(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}

	var req dto.CreateAcademicYearRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.years.Create(c.Request.Context(), actorID, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, utils.CodeCreated, "Tahun ajaran berhasil dibuat", result)
}

// Update menangani PATCH /api/v1/academic-years/{id}.
func (h *AcademicYearController) Update(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.UpdateAcademicYearRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.years.Update(c.Request.Context(), actorID, id, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Tahun ajaran berhasil diperbarui", result)
}

// Delete menangani DELETE /api/v1/academic-years/{id} (khusus Admin).
func (h *AcademicYearController) Delete(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	if err := h.years.Delete(c.Request.Context(), actorID, id); err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Tahun ajaran berhasil dihapus", nil)
}

// parsePageAndLimit membaca query page & limit standar (maks 100).
func parsePageAndLimit(c *gin.Context) (int, int, bool) {
	page, err := parsePositiveQueryInt(c, "page", 1)
	if err != nil {
		c.Error(err)
		return 0, 0, false
	}
	limit, err := parsePositiveQueryInt(c, "limit", 20)
	if err != nil {
		c.Error(err)
		return 0, 0, false
	}
	if limit > 100 {
		c.Error(utils.NewValidationError([]utils.FieldError{{
			Field:   "limit",
			Code:    utils.FieldInvalidLimit,
			Message: "limit maksimal 100",
		}}))
		return 0, 0, false
	}
	return page, limit, true
}

// parseOptionalActiveFilter membaca filter aktif dari query `is_active`
// (true/false) atau `status` (active/inactive).
func parseOptionalActiveFilter(c *gin.Context) (*bool, bool) {
	if raw := strings.TrimSpace(c.Query("is_active")); raw != "" {
		val, err := parseBoolQuery(c, "is_active", false)
		if err != nil {
			c.Error(err)
			return nil, false
		}
		return &val, true
	}
	if status := strings.ToLower(strings.TrimSpace(c.Query("status"))); status != "" {
		switch status {
		case "active":
			t := true
			return &t, true
		case "inactive":
			f := false
			return &f, true
		default:
			c.Error(utils.NewValidationError([]utils.FieldError{{
				Field:   "status",
				Code:    utils.FieldInvalidStatus,
				Message: "status harus active atau inactive",
			}}))
			return nil, false
		}
	}
	return nil, true
}

// parseSearchQuery membaca query pencarian dari `q` atau `search`.
func parseSearchQuery(c *gin.Context) string {
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		return q
	}
	return strings.TrimSpace(c.Query("search"))
}
