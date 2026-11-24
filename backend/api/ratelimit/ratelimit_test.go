package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiterDisabledAlwaysAllows(t *testing.T) {
	rl := NewRateLimiter(false, 1, 5)
	for range 5 {
		allowed, wait := rl.CheckLimit("10.0.0.1")
		assert.True(t, allowed)
		assert.Equal(t, 0, wait)
	}
}

func TestRateLimiterEnforcesWindowAndPerClientIsolation(t *testing.T) {
	rl := NewRateLimiter(true, 2, 1)

	allowed, _ := rl.CheckLimit("10.0.0.1")
	assert.True(t, allowed)
	allowed, _ = rl.CheckLimit("10.0.0.1")
	assert.True(t, allowed)

	allowed, wait := rl.CheckLimit("10.0.0.1")
	assert.False(t, allowed)
	assert.Greater(t, wait, 0)

	// Different client must not be affected
	allowed, _ = rl.CheckLimit("10.0.0.2")
	assert.True(t, allowed)

	// Expire old attempts and verify client is allowed again
	rl.mutex.Lock()
	rl.attempts["10.0.0.1"] = []time.Time{time.Now().Add(-2 * time.Minute)}
	rl.mutex.Unlock()

	allowed, _ = rl.CheckLimit("10.0.0.1")
	assert.True(t, allowed)
}
