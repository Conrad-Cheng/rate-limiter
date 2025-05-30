package main

import (
	"fmt"
	"net/http"
	"time"
)

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
