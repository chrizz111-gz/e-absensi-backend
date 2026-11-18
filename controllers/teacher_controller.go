package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/dto"
	"e-absensi-backend/services"
	"e-absensi-backend/utils"
)

// TeacherController menangani endpoint F3.2 Data Guru.
type TeacherController struct {
	teachers *services.TeacherService
}

// NewTeacherController membuat TeacherController baru.
func NewTeacherController(teachers *services.TeacherService) *TeacherController {
	return &TeacherController{teachers: teachers}
}

// Candidates menangani GET /api/v1/teachers/candidates (dropdown akun guru_mapel
// yang belum punya profil guru).
func (h *TeacherController) Candidates(c *gin.Context) {
	items, err := h.teachers.ListCandidates(c.Request.Context(), parseSearchQuery(c))
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar kandidat akun guru berhasil diambil", gin.H{
		"items": items,
	})
}

// List menangani GET /api/v1/teachers.
func (h *TeacherController) List(c *gin.Context) {
	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	isActive, ok := parseOptionalActiveFilter(c)
	if !ok {
		return
	}
	subjectID, ok := parseOptionalInt64Query(c, "subject_id")
	if !ok {
		return
	}

	result, err := h.teachers.List(c.Request.Context(), dto.TeacherListFilter{
		Page:      page,
		Limit:     limit,
		Q:         parseSearchQuery(c),
		SubjectID: subjectID,
		IsActive:  isActive,
	})
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar guru berhasil diambil", result)
}

// Get menangani GET /api/v1/teachers/{id}.
func (h *TeacherController) Get(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	result, err := h.teachers.Get(c.Request.Context(), id)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Detail guru berhasil diambil", result)
}

// Create menangani POST /api/v1/teachers.
func (h *TeacherController) Create(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}

	var req dto.CreateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.teachers.Create(c.Request.Context(), actorID, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, utils.CodeCreated, "Data guru berhasil dibuat", result)
}

// Update menangani PATCH /api/v1/teachers/{id}.
func (h *TeacherController) Update(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.UpdateTeacherRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.teachers.Update(c.Request.Context(), actorID, id, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Data guru berhasil diperbarui", result)
}
