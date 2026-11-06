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

// UserController menangani endpoint admin untuk CRUD user dan role assignment.
type UserController struct {
	users *services.UserService
}

// NewUserController membuat UserController baru.
func NewUserController(users *services.UserService) *UserController {
	return &UserController{users: users}
}

// List menangani GET /api/v1/users.
func (h *UserController) List(c *gin.Context) {
	filter, err := parseUserListFilter(c)
	if err != nil {
		c.Error(err)
		return
	}

	result, err := h.users.List(c.Request.Context(), filter)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar user berhasil diambil", result)
}

// Get menangani GET /api/v1/users/{id}.
func (h *UserController) Get(c *gin.Context) {
	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	result, err := h.users.Get(c.Request.Context(), userID)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Detail user berhasil diambil", result)
}

// Create menangani POST /api/v1/users.
func (h *UserController) Create(c *gin.Context) {
	var req dto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.users.Create(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, utils.CodeCreated, "User berhasil dibuat", result)
}

// SetStatus menangani PATCH /api/v1/users/{id}/status.
func (h *UserController) SetStatus(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.users.SetStatus(c.Request.Context(), actorID, userID, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Status user berhasil diperbarui", result)
}

// ReplaceRoles menangani PUT /api/v1/users/{id}/roles.
func (h *UserController) ReplaceRoles(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.UpdateUserRolesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}
	if req.Roles == nil {
		c.Error(utils.NewValidationError([]utils.FieldError{{
			Field:   "roles",
			Code:    utils.FieldRequired,
			Message: "roles wajib diisi (boleh array kosong)",
		}}))
		return
	}

	result, err := h.users.ReplaceRoles(c.Request.Context(), actorID, userID, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Role user berhasil diperbarui", result)
}

// Update menangani PATCH /api/v1/users/{id}.
func (h *UserController) Update(c *gin.Context) {
	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(utils.NewValidationError(utils.FormatValidationErrors(err)))
		return
	}

	result, err := h.users.Update(c.Request.Context(), userID, req)
	if err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "User berhasil diperbarui", result)
}

// Delete menangani DELETE /api/v1/users/{id}.
func (h *UserController) Delete(c *gin.Context) {
	actorID, ok := currentActorID(c)
	if !ok {
		return
	}
	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	if err := h.users.Delete(c.Request.Context(), actorID, userID); err != nil {
		c.Error(err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "User berhasil dihapus", nil)
}

// parseUserListFilter membaca query GET /users dengan default page=1, limit=20.
func parseUserListFilter(c *gin.Context) (dto.UserListFilter, error) {
	page, err := parsePositiveQueryInt(c, "page", 1)
	if err != nil {
		return dto.UserListFilter{}, err
	}
	limit, err := parsePositiveQueryInt(c, "limit", 20)
	if err != nil {
		return dto.UserListFilter{}, err
	}
	if limit > 100 {
		return dto.UserListFilter{}, utils.NewValidationError([]utils.FieldError{{
			Field:   "limit",
			Code:    utils.FieldInvalidLimit,
			Message: "limit maksimal 100",
		}})
	}

	roles := append([]string(nil), c.QueryArray("role[]")...)
	roles = append(roles, c.QueryArray("role")...)

	return dto.UserListFilter{
		Page:   page,
		Limit:  limit,
		Q:      strings.TrimSpace(c.Query("q")),
		Roles:  roles,
		Status: strings.ToLower(strings.TrimSpace(c.Query("status"))),
	}, nil
}

// parsePositiveQueryInt membaca query integer positif dengan fallback default.
func parsePositiveQueryInt(c *gin.Context, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		code := utils.FieldInvalidPage
		if key == "limit" {
			code = utils.FieldInvalidLimit
		}
		return 0, utils.NewValidationError([]utils.FieldError{{
			Field:   key,
			Code:    code,
			Message: key + " harus berupa bilangan bulat minimal 1",
		}})
	}
	return value, nil
}

// parseUserID membaca path parameter id dan mengembalikan 422 bila tidak valid.
func parseUserID(c *gin.Context) (int64, bool) {
	userID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || userID < 1 {
		c.Error(utils.NewValidationError([]utils.FieldError{{
			Field:   "id",
			Code:    utils.FieldInvalidID,
			Message: "id user tidak valid",
		}}))
		return 0, false
	}
	return userID, true
}

// currentActorID mengambil id user yang sedang login.
func currentActorID(c *gin.Context) (int64, bool) {
	actorID, ok := middlewares.CurrentUserID(c)
	if !ok {
		c.Error(utils.ErrUnauthorized)
		return 0, false
	}
	return actorID, true
}
