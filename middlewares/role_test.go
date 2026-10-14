package middlewares

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasAnyRole(t *testing.T) {
	assert.True(t, hasAnyRole([]string{"admin"}, []string{"admin", "guru_mapel"}))
	assert.True(t, hasAnyRole([]string{"siswa", "wali_kelas"}, []string{"wali_kelas"}))
	assert.False(t, hasAnyRole([]string{"siswa"}, []string{"admin"}))
	assert.False(t, hasAnyRole(nil, []string{"admin"}))
	assert.True(t, hasAnyRole([]string{"siswa"}, nil))
}
