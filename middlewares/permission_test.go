package middlewares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"e-absensi-backend/models"
)

type fakePermissionChecker struct {
	allowed    bool
	err        error
	userID     int64
	permission string
	calls      int
}

func (f *fakePermissionChecker) UserHasPermission(userID int64, permissionCode string) (bool, error) {
	f.calls++
	f.userID = userID
	f.permission = permissionCode
	return f.allowed, f.err
}

func TestRequirePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name       string
		user       *models.User
		allowed    bool
		checkerErr error
		wantStatus int
		wantCalls  int
	}{
		{
			name:       "tanpa user terautentikasi",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "tidak punya permission",
			user:       &models.User{ID: 9},
			allowed:    false,
			wantStatus: http.StatusForbidden,
			wantCalls:  1,
		},
		{
			name:       "punya permission",
			user:       &models.User{ID: 9},
			allowed:    true,
			wantStatus: http.StatusOK,
			wantCalls:  1,
		},
		{
			name:       "query permission gagal",
			user:       &models.User{ID: 9},
			checkerErr: errors.New("db error"),
			wantStatus: http.StatusInternalServerError,
			wantCalls:  1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checker := &fakePermissionChecker{allowed: tc.allowed, err: tc.checkerErr}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				if tc.user != nil {
					c.Set(ContextUserKey, tc.user)
				}
				c.Next()
			})
			router.GET("/protected", requirePermission(checker, models.PermissionAccountView), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			router.ServeHTTP(recorder, req)

			assert.Equal(t, tc.wantStatus, recorder.Code)
			assert.Equal(t, tc.wantCalls, checker.calls)
			if tc.wantCalls > 0 {
				assert.Equal(t, int64(9), checker.userID)
				assert.Equal(t, models.PermissionAccountView, checker.permission)
			}
		})
	}
}
