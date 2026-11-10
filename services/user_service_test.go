package services

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"

	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

func TestNormalizeRoleNames(t *testing.T) {
	got := normalizeRoleNames([]string{" Admin ", "guru_mapel", "ADMIN", "", "  "})
	assert.Equal(t, []string{"admin", "guru_mapel"}, got)
}

func TestIsStudent(t *testing.T) {
	assert.True(t, isStudent([]models.Role{{Name: models.RoleSiswa, Category: "student"}}))
	assert.True(t, isStudent([]models.Role{{Name: models.RoleSiswaInti}}))
	assert.False(t, isStudent([]models.Role{{Name: models.RoleGuruMapel, Category: "staff"}}))
	assert.False(t, isStudent(nil))
}

func TestCountActiveAdmins(t *testing.T) {
	admins := []repositories.AdminLock{
		{ID: 1, IsActive: true},
		{ID: 2, IsActive: false},
		{ID: 3, IsActive: true},
	}
	assert.Equal(t, 2, countActiveAdmins(admins))
	assert.Equal(t, 0, countActiveAdmins(nil))
}

func TestMapUserWriteError(t *testing.T) {
	emailErr := &pgconn.PgError{Code: "23505", ConstraintName: "uq_users_email_lower"}
	appErr := mapUserWriteError(emailErr)
	assert.NotNil(t, appErr)
	assert.Equal(t, http.StatusConflict, appErr.HTTPStatus)
	assert.Equal(t, utils.CodeEmailAlreadyExists, appErr.Code)
	assert.Equal(t, utils.FieldEmailAlreadyExists, appErr.Errors[0].Code)

	assert.Nil(t, mapUserWriteError(errors.New("bukan unique violation")))
}

func TestIsForeignKeyViolation(t *testing.T) {
	assert.True(t, isForeignKeyViolation(&pgconn.PgError{Code: "23503"}))
	assert.True(t, isForeignKeyViolation(&pgconn.PgError{Code: "23001"}))
	assert.False(t, isForeignKeyViolation(errors.New("bukan fk violation")))
}

func TestValidateRoleCategory(t *testing.T) {
	assert.NoError(t, validateRoleCategory([]models.Role{{Name: models.RoleGuruMapel, Category: "staff"}}))
	assert.NoError(t, validateRoleCategory([]models.Role{{Name: models.RoleSiswa, Category: "student"}}))
	assert.NoError(t, validateRoleCategory([]models.Role{
		{Name: models.RoleSiswa, Category: "student"},
		{Name: models.RoleSiswaInti, Category: "student"},
	}))

	appErr := validateRoleCategory([]models.Role{
		{Name: models.RoleGuruMapel, Category: "staff"},
		{Name: models.RoleSiswa, Category: "student"},
	})
	assert.IsType(t, &utils.AppError{}, appErr)
	assert.Equal(t, utils.CodeInvalidRoleCombination, appErr.(*utils.AppError).Code)

	appErr = validateRoleCategory([]models.Role{{Name: models.RoleSiswaInti, Category: "student"}})
	assert.IsType(t, &utils.AppError{}, appErr)
	assert.Equal(t, utils.CodeInvalidRoleCombination, appErr.(*utils.AppError).Code)
}

func TestDiffRoleNames(t *testing.T) {
	added, removed := diffRoleNames(
		[]models.Role{{Name: models.RoleAdmin}, {Name: models.RoleGuruMapel}},
		[]models.Role{{Name: models.RoleAdmin}, {Name: models.RoleWakaKurikulum}},
	)
	assert.Equal(t, []string{models.RoleWakaKurikulum}, added)
	assert.Equal(t, []string{models.RoleGuruMapel}, removed)
}

func TestAuditUserSnapshotExcludesSensitiveFields(t *testing.T) {
	password := "rahasia-password"
	googleID := "google-sub-123"
	user := &models.User{
		ID:           1,
		Name:         "Budi",
		Email:        "budi@example.com",
		PasswordHash: &password,
		GoogleID:     &googleID,
		IsActive:     true,
	}
	snapshot, err := auditUserSnapshot(user, []models.Role{{Name: models.RoleAdmin}})
	assert.NoError(t, err)
	assert.NotContains(t, string(snapshot), password)
	assert.NotContains(t, string(snapshot), googleID)
	assert.Contains(t, string(snapshot), models.RoleAdmin)

	var decoded map[string]interface{}
	assert.NoError(t, json.Unmarshal(snapshot, &decoded))
	assert.Equal(t, float64(1), decoded["id"])
}

func TestAuditUserUpdateSnapshotHasBeforeAndAfter(t *testing.T) {
	before := map[string]interface{}{"id": 1, "name": "Lama", "email": "lama@example.com"}
	after := map[string]interface{}{"id": 1, "name": "Baru", "email": "baru@example.com"}
	snapshot, err := auditUserUpdateSnapshot(before, after)
	assert.NoError(t, err)
	assert.Contains(t, string(snapshot), `"before"`)
	assert.Contains(t, string(snapshot), `"after"`)
	assert.Contains(t, string(snapshot), "Lama")
	assert.Contains(t, string(snapshot), "Baru")
	assert.NotContains(t, string(snapshot), "password_hash")
	assert.NotContains(t, string(snapshot), "google_id")
}
