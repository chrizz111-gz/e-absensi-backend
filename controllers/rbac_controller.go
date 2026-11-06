package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"e-absensi-backend/dto"
	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

// RBACController menangani endpoint read-only role dan matriks permission
// (Epic 2 F2.7).
type RBACController struct {
	roles       *repositories.RoleRepository
	permissions *repositories.PermissionRepository
}

// NewRBACController membuat RBACController baru.
func NewRBACController(db *gorm.DB) *RBACController {
	return &RBACController{
		roles:       repositories.NewRoleRepository(db),
		permissions: repositories.NewPermissionRepository(db),
	}
}

// ListRoles menangani GET /api/v1/roles.
func (h *RBACController) ListRoles(c *gin.Context) {
	roles, err := h.roles.List()
	if err != nil {
		c.Error(utils.ErrInternal)
		return
	}

	data := make([]dto.RoleResponse, 0, len(roles))
	for _, role := range roles {
		data = append(data, toRoleResponse(role))
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Daftar role berhasil diambil", dto.RoleListResponse{Roles: data})
}

// PermissionMatrix menangani GET /api/v1/permissions/matrix.
func (h *RBACController) PermissionMatrix(c *gin.Context) {
	roles, err := h.roles.List()
	if err != nil {
		c.Error(utils.ErrInternal)
		return
	}
	permissions, err := h.permissions.List()
	if err != nil {
		c.Error(utils.ErrInternal)
		return
	}
	grants, err := h.permissions.ListGrants()
	if err != nil {
		c.Error(utils.ErrInternal)
		return
	}

	roleData := make([]dto.RoleResponse, 0, len(roles))
	for _, role := range roles {
		roleData = append(roleData, toRoleResponse(role))
	}

	permissionData := make([]dto.PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		permissionData = append(permissionData, dto.PermissionResponse{
			ID:          permission.ID,
			Code:        permission.Code,
			Module:      permission.Module,
			Description: permission.Description,
		})
	}

	grantData := make([]dto.PermissionGrantResponse, 0, len(grants))
	for _, grant := range grants {
		grantData = append(grantData, dto.PermissionGrantResponse{
			RoleID:       grant.RoleID,
			PermissionID: grant.PermissionID,
		})
	}

	utils.SuccessResponse(c, http.StatusOK, utils.CodeSuccess, "Matriks hak akses berhasil diambil", dto.PermissionMatrixResponse{
		Roles:       roleData,
		Permissions: permissionData,
		Grants:      grantData,
	})
}

// toRoleResponse memetakan model role ke DTO aman.
func toRoleResponse(role models.Role) dto.RoleResponse {
	return dto.RoleResponse{
		ID:           role.ID,
		Name:         role.Name,
		Label:        role.Label,
		Category:     role.Category,
		IsAssignable: role.IsAssignable,
		Description:  role.Description,
	}
}
