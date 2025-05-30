package main

import (
	"sync"
	"time"
)

type RateLimiter struct {
	rate     int
	capacity int
	tokens   int
	mutex    sync.Mutex
	ticker   *time.Ticker
	quit     chan struct{}
	lastSeen time.Time
}

func NewRateLimiter(rate, capacity int) *RateLimiter {
	rl := &RateLimiter{
		rate:     rate,
		capacity: capacity,
		tokens:   capacity,
		ticker:   time.NewTicker(time.Second / time.Duration(rate)),
		quit:     make(chan struct{}),
		lastSeen: time.Now(),
	}
	go rl.refill()
	return rl
}

func (rl *RateLimiter) refill() {
	for {
		select {
		case <-rl.ticker.C:
			rl.mutex.Lock()
			if rl.tokens < rl.capacity {
				rl.tokens++
			}
			rl.mutex.Unlock()
		case <-rl.quit:
			rl.ticker.Stop()
			return
		}
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()
	rl.lastSeen = time.Now()

	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	return false
}

func (rl *RateLimiter) Stop() {
	close(rl.quit)
}
