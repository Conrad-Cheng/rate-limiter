package main

import (
	"sync"
	"time"
)

type LimiterStore struct {
	clients  map[string]*RateLimiter
	mutex    sync.Mutex
	rate     int
	capacity int
	ttl      time.Duration
}

func NewLimiterStore(rate, capacity int, ttl time.Duration) *LimiterStore {
	store := &LimiterStore{
		clients:  make(map[string]*RateLimiter),
		rate:     rate,
		capacity: capacity,
		ttl:      ttl,
	}
	go store.cleanup()
	return store
}

func (s *LimiterStore) GetLimiter(ip string) *RateLimiter {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if limiter, ok := s.clients[ip]; ok {
		return limiter
	}

	limiter := NewRateLimiter(s.rate, s.capacity)
	s.clients[ip] = limiter
	return limiter
}

func (s *LimiterStore) cleanup() {
	for {
		time.Sleep(s.ttl)
		s.mutex.Lock()
		now := time.Now()
		for ip, limiter := range s.clients {
			limiter.mutex.Lock()
			inactive := now.Sub(limiter.lastSeen) > s.ttl
			limiter.mutex.Unlock()

			if inactive {
				limiter.Stop()
				delete(s.clients, ip)
			}
		}
		s.mutex.Unlock()
	}
}
