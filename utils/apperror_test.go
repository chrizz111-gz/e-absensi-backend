package utils

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPasswordSameErrorUsesBadRequest(t *testing.T) {
	assert.Equal(t, http.StatusBadRequest, ErrPasswordSame.HTTPStatus)
	assert.Equal(t, CodePasswordSame, ErrPasswordSame.Code)
}
