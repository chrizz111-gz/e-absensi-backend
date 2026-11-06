package services

import (
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
	assert.Equal(t, utils.FieldEmailTaken, appErr.Errors[0].Code)

	usernameErr := &pgconn.PgError{Code: "23505", ConstraintName: "uq_users_username"}
	appErr = mapUserWriteError(usernameErr)
	assert.NotNil(t, appErr)
	assert.Equal(t, utils.FieldUsernameTaken, appErr.Errors[0].Code)

	assert.Nil(t, mapUserWriteError(errors.New("bukan unique violation")))
}

func TestIsForeignKeyViolation(t *testing.T) {
	assert.True(t, isForeignKeyViolation(&pgconn.PgError{Code: "23503"}))
	assert.True(t, isForeignKeyViolation(&pgconn.PgError{Code: "23001"}))
	assert.False(t, isForeignKeyViolation(errors.New("bukan fk violation")))
}
