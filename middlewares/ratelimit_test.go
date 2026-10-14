package middlewares

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIPLimiterBurst(t *testing.T) {
	limiter := newIPLimiter(1, 3)

	allowed := 0
	for i := 0; i < 5; i++ {
		if limiter.allow("1.2.3.4") {
			allowed++
		}
	}
	assert.Equal(t, 3, allowed)
}

func TestIPLimiterSeparateKeys(t *testing.T) {
	limiter := newIPLimiter(1, 1)

	assert.True(t, limiter.allow("10.0.0.1"))
	assert.False(t, limiter.allow("10.0.0.1"))
	assert.True(t, limiter.allow("10.0.0.2"))
}

func TestIPLimiterDisabled(t *testing.T) {
	limiter := newIPLimiter(0, 0)

	for i := 0; i < 100; i++ {
		assert.True(t, limiter.allow("x"))
	}
}
