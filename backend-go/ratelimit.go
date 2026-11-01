package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// TokenBucket represents an individual rate-limit bucket
type TokenBucket struct {
	tokens         float64
	maxTokens      float64
	refillRate     float64 // tokens added per second
	lastRefillTime time.Time
}

func (tb *TokenBucket) allow() bool {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime).Seconds()
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefillTime = now

	if tb.tokens >= 1.0 {
		tb.tokens -= 1.0
		return true
	}
	return false
}

// MemoryRateLimiter manages token buckets for clients/keys
type MemoryRateLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*TokenBucket
	maxTokens  float64
	refillRate float64
}

// NewRateLimiter creates a thread-safe token bucket limiter
// maxTokens: burst capacity, refillRate: allowed sustained requests per second
func NewRateLimiter(maxTokens float64, refillRate float64) *MemoryRateLimiter {
	limiter := &MemoryRateLimiter{
		buckets:    make(map[string]*TokenBucket),
		maxTokens:  maxTokens,
		refillRate: refillRate,
	}

	// Periodic cleanup of idle buckets to avoid memory growth
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			limiter.cleanup()
		}
	}()

	return limiter
}

func (rl *MemoryRateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.buckets[key]
	if !exists {
		bucket = &TokenBucket{
			tokens:         rl.maxTokens,
			maxTokens:      rl.maxTokens,
			refillRate:     rl.refillRate,
			lastRefillTime: time.Now(),
		}
		rl.buckets[key] = bucket
	}

	return bucket.allow()
}

func (rl *MemoryRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, bucket := range rl.buckets {
		// If bucket has been idle for more than 10 minutes and is full, clean it up
		if now.Sub(bucket.lastRefillTime) > 10*time.Minute && bucket.tokens >= bucket.maxTokens {
			delete(rl.buckets, key)
		}
	}
}

// Global Limiter Instances
// Ingress: 200 burst, 100 sustained req/sec per channel/client
var GlobalIngressLimiter = NewRateLimiter(200, 100)

// Login: 10 burst, 0.2 sustained req/sec (1 request every 5s) to thwart brute-force attacks
var GlobalLoginLimiter = NewRateLimiter(10, 0.2)

// Management API: 120 burst, 40 sustained req/sec per client IP
var GlobalAPILimiter = NewRateLimiter(120, 40)

// RateLimitMiddleware enforces token-bucket rate limiting based on client IP or route
func RateLimitMiddleware(limiter *MemoryRateLimiter, prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("%s:%s", prefix, clientIP)

		if !limiter.Allow(key) {
			c.Header("Retry-After", "2")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded (Too Many Requests). System protected against overload.",
				"status_code": http.StatusTooManyRequests,
				"client_ip":   clientIP,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// IngressRateLimitMiddleware enforces rate limiting per channel route
func IngressRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		routeID := c.Param("routeId")
		clientIP := c.ClientIP()
		key := fmt.Sprintf("ingress:%s:%s", routeID, clientIP)

		if !GlobalIngressLimiter.Allow(key) {
			c.Header("Retry-After", "1")
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":    "Channel Ingress throughput limit exceeded. Please throttle client ingestion.",
				"route_id": routeID,
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// MaxPayloadSizeMiddleware rejects requests exceeding the specified maximum body size
func MaxPayloadSizeMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":     fmt.Sprintf("Payload size (%d bytes) exceeds maximum allowed limit (%d bytes)", c.Request.ContentLength, maxBytes),
				"max_bytes": maxBytes,
			})
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
