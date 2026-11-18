package middlewares

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"e-absensi-backend/models"
	"e-absensi-backend/repositories"
	"e-absensi-backend/utils"
)

type fakeAuthUserLoader struct {
	user  *models.User
	roles []models.Role
	err   error
}

func (f *fakeAuthUserLoader) FindByID(id int64) (*models.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.user == nil || f.user.ID != id {
		return nil, repositories.ErrNotFound
	}
	return f.user, nil
}

func (f *fakeAuthUserLoader) GetRoles(userID int64) ([]models.Role, error) {
	return f.roles, nil
}

func TestAuthRequiredRejectsTokenIssuedBeforePasswordChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_SECRET", "test-secret-auth-middleware")

	token, err := utils.GenerateAccessToken(7, "user@test.com", []string{models.RoleSiswa})
	require.NoError(t, err)

	changedAt := time.Now().Add(2 * time.Second)
	loader := &fakeAuthUserLoader{
		user: &models.User{
			ID:                7,
			Email:             "user@test.com",
			IsActive:          true,
			PasswordChangedAt: &changedAt,
		},
		roles: []models.Role{{ID: 1, Name: models.RoleSiswa}},
	}

	router := gin.New()
	router.GET("/me", authRequired(loader), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var body utils.APIResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.False(t, body.Success)
	assert.Equal(t, utils.CodeSessionRevoked, body.Code)
}

func TestAuthRequiredAllowsTokenIssuedAfterPasswordChange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("JWT_SECRET", "test-secret-auth-middleware")

	changedAt := time.Now().Add(-5 * time.Second)
	token, err := utils.GenerateAccessToken(7, "user@test.com", []string{models.RoleSiswa})
	require.NoError(t, err)

	loader := &fakeAuthUserLoader{
		user: &models.User{
			ID:                7,
			Email:             "user@test.com",
			IsActive:          true,
			PasswordChangedAt: &changedAt,
		},
		roles: []models.Role{{ID: 1, Name: models.RoleSiswa}},
	}

	router := gin.New()
	router.GET("/me", authRequired(loader), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

