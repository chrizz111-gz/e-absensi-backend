package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/dto"
	"e-absensi-backend/services"
	"e-absensi-backend/utils"
)

// SubjectController menangani endpoint F3.4 Mata Pelajaran.
type SubjectController struct {
	subjects *services.SubjectService
}

// NewSubjectController membuat SubjectController baru.
func NewSubjectController(subjects *services.SubjectService) *SubjectController {
	return &SubjectController{subjects: subjects}
}

// List menangani GET /api/v1/subjects.
func (h *SubjectController) List(c *gin.Context) {
	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	isActive, ok := parseOptionalActiveFilter(c)
	if !ok {
		return
	}

	result, err := h.subjects.List(c.Request.Context(), dto.SubjectListFilter{
		Page:     page,
		Limit:    limit,
		Q:        parseSearchQuery(c),
		IsActive: isActive,
	})
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar mata pelajaran berhasil diambil", result)
}

// Get menangani GET /api/v1/subjects/{id}.
func (h *SubjectController) Get(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	result, err := h.subjects.Get(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Detail mata pelajaran berhasil diambil", result)
}

// Create menangani POST /api/v1/subjects.
func (h *SubjectController) Create(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}

	var req dto.CreateSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.subjects.Create(c.Request.Context(), actorID, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, utils.CodeCreated, "Mata pelajaran berhasil dibuat", result)
}

// Update menangani PATCH /api/v1/subjects/{id}.
func (h *SubjectController) Update(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.UpdateSubjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.subjects.Update(c.Request.Context(), actorID, id, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Mata pelajaran berhasil diperbarui", result)
}

// Delete menangani DELETE /api/v1/subjects/{id}.
func (h *SubjectController) Delete(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	if err := h.subjects.Delete(c.Request.Context(), actorID, id); err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Mata pelajaran berhasil dihapus", nil)
}
