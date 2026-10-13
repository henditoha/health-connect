package main

import (
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Active source connections per route (e.g. TCP clients currently connected).
var (
	connMu     sync.Mutex
	connCounts = make(map[string]int)

	// In-memory cache to prevent database overload from frequent stats polling
	statsCacheMu   sync.RWMutex
	cachedStats    map[string]*RouteStats
	lastStatsFetch time.Time
	statsTTL       = 4 * time.Second
)

func connOpened(routeID string) {
	connMu.Lock()
	connCounts[routeID]++
	connMu.Unlock()
}

func connClosed(routeID string) {
	connMu.Lock()
	if connCounts[routeID] > 0 {
		connCounts[routeID]--
	}
	connMu.Unlock()
}

type RouteStats struct {
	Received    int `json:"received"`
	Filtered    int `json:"filtered"`
	Queue       int `json:"queue"`
	Sent        int `json:"sent"`
	Error       int `json:"error"`
	Connections int `json:"connections"`
}

func getOrCreateStats(stats map[string]*RouteStats, id string) *RouteStats {
	s, ok := stats[id]
	if !ok {
		s = &RouteStats{}
		stats[id] = s
	}
	return s
}

func applyLogStatus(s *RouteStats, status string, count int) {
	s.Received += count
	switch status {
	case "SUCCESS":
		s.Sent += count
	case "FILTERED":
		s.Filtered += count
	case "ERROR":
		s.Error += count
	}
}

func loadLogStats(stats map[string]*RouteStats) {
	rows, err := db.Query("SELECT route_id, status, COUNT(*) FROM message_logs GROUP BY route_id, status")
	if err != nil {
		log.Println("Error querying message log stats:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, status string
		var n int
		if err := rows.Scan(&id, &status, &n); err != nil {
			continue
		}
		s := getOrCreateStats(stats, id)
		applyLogStatus(s, status, n)
	}

	if err := rows.Err(); err != nil {
		log.Println("Error iterating message log stats rows:", err)
	}
}

func loadQueueStats(stats map[string]*RouteStats) {
	rows, err := db.Query("SELECT route_id, COUNT(*) FROM queue_messages WHERE status IN ('pending', 'processing', 'failed') GROUP BY route_id")
	if err != nil {
		log.Println("Error querying queue stats:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			continue
		}
		getOrCreateStats(stats, id).Queue = n
	}

	if err := rows.Err(); err != nil {
		log.Println("Error iterating queue stats rows:", err)
	}
}

func loadConnectionStats(stats map[string]*RouteStats) {
	connMu.Lock()
	defer connMu.Unlock()

	for id, n := range connCounts {
		if n > 0 {
			getOrCreateStats(stats, id).Connections = n
		}
	}
}

func copyStatsMap(src map[string]*RouteStats) map[string]*RouteStats {
	dst := make(map[string]*RouteStats, len(src))
	for k, v := range src {
		if v != nil {
			dst[k] = &RouteStats{
				Received:    v.Received,
				Filtered:    v.Filtered,
				Queue:       v.Queue,
				Sent:        v.Sent,
				Error:       v.Error,
				Connections: v.Connections,
			}
		}
	}
	return dst
}

func getStats(c *gin.Context) {
	statsCacheMu.RLock()
	if cachedStats != nil && time.Since(lastStatsFetch) < statsTTL {
		copied := copyStatsMap(cachedStats)
		statsCacheMu.RUnlock()
		c.JSON(200, copied)
		return
	}
	statsCacheMu.RUnlock()

	statsCacheMu.Lock()
	defer statsCacheMu.Unlock()

	// Double check under write lock
	if cachedStats != nil && time.Since(lastStatsFetch) < statsTTL {
		c.JSON(200, copyStatsMap(cachedStats))
		return
	}

	stats := make(map[string]*RouteStats)
	loadLogStats(stats)
	loadQueueStats(stats)
	loadConnectionStats(stats)

	cachedStats = stats
	lastStatsFetch = time.Now()

	c.JSON(200, copyStatsMap(cachedStats))
}
