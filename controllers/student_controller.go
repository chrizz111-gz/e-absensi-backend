package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"e-absensi-backend/dto"
	"e-absensi-backend/middlewares"
	"e-absensi-backend/services"
	"e-absensi-backend/utils"
)

// StudentController menangani endpoint F3.1 Data Siswa.
type StudentController struct {
	students *services.StudentService
}

// NewStudentController membuat StudentController baru.
func NewStudentController(students *services.StudentService) *StudentController {
	return &StudentController{students: students}
}

// List menangani GET /api/v1/students.
func (h *StudentController) List(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	roles, _ := middlewares.CurrentRoles(c)

	page, limit, ok := parsePageAndLimit(c)
	if !ok {
		return
	}
	isActive, ok := parseOptionalActiveFilter(c)
	if !ok {
		return
	}
	classID, ok := parseOptionalInt64Query(c, "class_id")
	if !ok {
		return
	}
	unassigned, err := parseBoolQuery(c, "unassigned", false)
	if err != nil {
		c.Error(err)
		return
	}
	includeArchived, err := parseBoolQuery(c, "include_archived", false)
	if err != nil {
		c.Error(err)
		return
	}

	result, err := h.students.List(c.Request.Context(), actorID, roles, dto.StudentListFilter{
		Page:            page,
		Limit:           limit,
		Q:               parseSearchQuery(c),
		ClassID:         classID,
		Unassigned:      unassigned,
		IsActive:        isActive,
		IncludeArchived: includeArchived,
	})
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar siswa berhasil diambil", result)
}

// Get menangani GET /api/v1/students/{id}.
func (h *StudentController) Get(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	roles, _ := middlewares.CurrentRoles(c)

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	result, err := h.students.Get(c.Request.Context(), actorID, roles, id)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Detail siswa berhasil diambil", result)
}

// Update menangani PATCH /api/v1/students/{id}.
func (h *StudentController) Update(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.UpdateStudentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.students.Update(c.Request.Context(), actorID, id, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Data siswa berhasil diperbarui", result)
}
