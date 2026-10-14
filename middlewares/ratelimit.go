package middlewares

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"e-absensi-backend/utils"
)

// ipLimiter menyimpan token-bucket limiter per kunci (IP) beserta evakuasi
// entri yang lama tidak dipakai agar tidak tumbuh tanpa batas.
type ipLimiter struct {
	mu       sync.Mutex
	entries  map[string]*limiterEntry
	rps      rate.Limit
	burst    int
	ttl      time.Duration
	now      func() time.Time
	disabled bool
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// newIPLimiter membuat limiter per-IP. rps<=0 berarti nonaktif (selalu izinkan).
func newIPLimiter(rps float64, burst int) *ipLimiter {
	l := &ipLimiter{
		entries: make(map[string]*limiterEntry),
		rps:     rate.Limit(rps),
		burst:   burst,
		ttl:     10 * time.Minute,
		now:     time.Now,
	}
	if rps <= 0 {
		l.disabled = true
		return l
	}
	if burst <= 0 {
		burst = int(rps)
		if burst < 1 {
			burst = 1
		}
		l.burst = burst
	}
	go l.cleanup()
	return l
}

// allow menentukan apakah request dengan kunci (IP) tertentu diizinkan.
func (l *ipLimiter) allow(key string) bool {
	if l.disabled {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok {
		entry = &limiterEntry{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.entries[key] = entry
	}
	entry.lastSeen = l.now()
	return entry.limiter.Allow()
}

// cleanup berkala membuang entri yang sudah lama tidak diakses.
func (l *ipLimiter) cleanup() {
	ticker := time.NewTicker(l.ttl)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := l.now().Add(-l.ttl)
		l.mu.Lock()
		for key, entry := range l.entries {
			if entry.lastSeen.Before(cutoff) {
				delete(l.entries, key)
			}
		}
		l.mu.Unlock()
	}
}

// RateLimit membatasi jumlah request per IP untuk endpoint sensitif. rps<=0
// menonaktifkan pembatasan.
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	limiter := newIPLimiter(rps, burst)
	return func(c *gin.Context) {
		if !limiter.allow(c.ClientIP()) {
			c.Header("Retry-After", "60")
			utils.AbortWithError(c, http.StatusTooManyRequests, utils.CodeRateLimited, "Terlalu banyak permintaan, silakan coba lagi nanti")
			return
		}
		c.Next()
	}
}
