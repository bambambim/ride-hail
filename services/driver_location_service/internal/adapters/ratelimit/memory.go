package ratelimit

import (
	"context"
	"sync"
	"time"

	"ride-hail/services/driver_location_service/internal/ports"
)

// MemoryRateLimiter implements a simple in-memory rate limiter
type MemoryRateLimiter struct {
	mu       sync.RWMutex
	buckets  map[string]*bucket
	rate     time.Duration
	capacity int
}

// bucket represents a token bucket for rate limiting
type bucket struct {
	tokens   int
	lastFill time.Time
	mu       sync.Mutex
}

// NewMemoryRateLimiter creates a new in-memory rate limiter
// rate: minimum duration between requests
// capacity: maximum burst capacity
func NewMemoryRateLimiter(rate time.Duration, capacity int) ports.RateLimiter {
	rl := &MemoryRateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		capacity: capacity,
	}

	// Start cleanup goroutine to remove old buckets
	go rl.cleanup()

	return rl
}

// Allow checks if an action is allowed for a given key
func (rl *MemoryRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return rl.AllowN(ctx, key, 1)
}

// AllowN checks if N actions are allowed for a given key
func (rl *MemoryRateLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	rl.mu.RLock()
	b, exists := rl.buckets[key]
	rl.mu.RUnlock()

	if !exists {
		// Create new bucket
		rl.mu.Lock()
		// Double-check after acquiring write lock
		b, exists = rl.buckets[key]
		if !exists {
			b = &bucket{
				tokens:   rl.capacity,
				lastFill: time.Now(),
			}
			rl.buckets[key] = b
		}
		rl.mu.Unlock()
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(b.lastFill)
	tokensToAdd := int(elapsed / rl.rate)

	if tokensToAdd > 0 {
		b.tokens += tokensToAdd
		if b.tokens > rl.capacity {
			b.tokens = rl.capacity
		}
		b.lastFill = now
	}

	// Check if we have enough tokens
	if b.tokens >= n {
		b.tokens -= n
		return true, nil
	}

	return false, nil
}

// Reset resets the rate limit for a given key
func (rl *MemoryRateLimiter) Reset(ctx context.Context, key string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if b, exists := rl.buckets[key]; exists {
		b.mu.Lock()
		b.tokens = rl.capacity
		b.lastFill = time.Now()
		b.mu.Unlock()
	}

	return nil
}

// cleanup removes stale buckets periodically
func (rl *MemoryRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanupStale()
	}
}

// cleanupStale removes buckets that haven't been used recently
func (rl *MemoryRateLimiter) cleanupStale() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	staleThreshold := time.Now().Add(-10 * time.Minute)

	for key, b := range rl.buckets {
		b.mu.Lock()
		if b.lastFill.Before(staleThreshold) {
			delete(rl.buckets, key)
		}
		b.mu.Unlock()
	}
}
