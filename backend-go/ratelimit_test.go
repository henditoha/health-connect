package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestTokenBucketRateLimiting(t *testing.T) {
	// Limiter with burst capacity of 3 and refill rate of 1 token/sec
	limiter := NewRateLimiter(3, 1)

	key := "test-client-1"

	// First 3 requests should be allowed (burst)
	for i := 0; i < 3; i++ {
		if !limiter.Allow(key) {
			t.Fatalf("Expected request %d to be allowed under burst capacity", i+1)
		}
	}

	// 4th immediate request should be rejected (bucket empty)
	if limiter.Allow(key) {
		t.Fatalf("Expected 4th request to be rejected due to rate limit")
	}

	// Sleep 1.1s for 1 token to refill
	time.Sleep(1100 * time.Millisecond)

	// Now 1 request should be allowed
	if !limiter.Allow(key) {
		t.Fatalf("Expected request after refill to be allowed")
	}

	// Next immediate request should be rejected again
	if limiter.Allow(key) {
		t.Fatalf("Expected immediate subsequent request to be rejected")
	}
}

func TestClientKeyIsolation(t *testing.T) {
	limiter := NewRateLimiter(1, 0.1)

	// Exhaust client A
	if !limiter.Allow("client-a") {
		t.Fatal("client-a should be allowed for initial token")
	}
	if limiter.Allow("client-a") {
		t.Fatal("client-a should be blocked on 2nd request")
	}

	// Client B should still be allowed
	if !limiter.Allow("client-b") {
		t.Fatal("client-b should not be blocked by client-a consumption")
	}
}

func TestMaxPayloadSizeMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(MaxPayloadSizeMiddleware(100)) // Max 100 bytes

	r.POST("/test-upload", func(c *gin.Context) {
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "received": buf.Len()})
	})

	// 1. Request within limit (50 bytes)
	smallBody := bytes.Repeat([]byte("a"), 50)
	req1, _ := http.NewRequest("POST", "/test-upload", bytes.NewReader(smallBody))
	req1.Header.Set("Content-Type", "application/octet-stream")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for payload within limit, got %d", w1.Code)
	}

	// 2. Request exceeding limit (200 bytes)
	largeBody := bytes.Repeat([]byte("b"), 200)
	req2, _ := http.NewRequest("POST", "/test-upload", bytes.NewReader(largeBody))
	req2.Header.Set("Content-Type", "application/octet-stream")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("Expected status 413 for payload exceeding limit, got %d", w2.Code)
	}
}
