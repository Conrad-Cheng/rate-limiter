package main

import (
    "fmt"
    "net"
    "net/http"
    "sync"
    "time"
)

// --- Token Bucket Rate Limiter ---

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

// --- Per-IP Rate Limiter Store ---

type LimiterStore struct {
    clients   map[string]*RateLimiter
    mutex     sync.Mutex
    rate      int
    capacity  int
    ttl       time.Duration
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

// --- Middleware ---

func RateLimitMiddleware(store *LimiterStore) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ip, _, err := net.SplitHostPort(r.RemoteAddr)
            if err != nil {
                http.Error(w, "Invalid IP", http.StatusInternalServerError)
                return
            }

            limiter := store.GetLimiter(ip)
            if !limiter.Allow() {
                http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

// --- Example Handler & Server ---

func helloHandler(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, IP: %s at %s\n", r.RemoteAddr, time.Now().Format(time.RFC3339))
}

func main() {
    store := NewLimiterStore(3, 5, 1*time.Minute) // 3 req/sec, burst 5, 1min cleanup TTL

    mux := http.NewServeMux()
    mux.Handle("/", RateLimitMiddleware(store)(http.HandlerFunc(helloHandler)))

    fmt.Println("Server running on http://localhost:8080")
    http.ListenAndServe(":8080", mux)
}
