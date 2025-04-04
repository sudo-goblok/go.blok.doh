package server

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Tambahan untuk rate limiter
type ClientLimiter struct {
	Limiter  *rate.Limiter
	LastSeen time.Time
}
type RateLimiterMap struct {
	clients map[string]*ClientLimiter
	mu      sync.Mutex
	limit   rate.Limit
	burst   int
}

func NewRateLimiterMap(limit rate.Limit, burst int) *RateLimiterMap {
	return &RateLimiterMap{
		clients: make(map[string]*ClientLimiter),
		limit:   limit,
		burst:   burst,
	}
}

func (rl *RateLimiterMap) GetLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	client, exists := rl.clients[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.limit, rl.burst)
		rl.clients[ip] = &ClientLimiter{
			Limiter:  limiter,
			LastSeen: time.Now(),
		}
		return limiter
	}

	client.LastSeen = time.Now()
	return client.Limiter
}
