package main

import (
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type QueueMessage struct {
	ID          string
	RouteID     string
	Payload     string
	Status      string
	RetryCount  int
	NextRetryAt string
	CreatedAt   string
}

const (
	MaxRetries = 5
	BaseDelay  = 5 * time.Second
)

func (e *IntegrationEngine) EnqueueMessage(routeID, payload string) error {
	id := uuid.New().String()
	_, err := db.Exec(
		"INSERT INTO queue_messages (id, route_id, payload, status, retry_count, next_retry_at, created_at) VALUES (?, ?, ?, 'pending', 0, ?, ?)",
		id, routeID, payload, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339),
	)
	return err
}

func (e *IntegrationEngine) StartQueueWorker() {
	go func() {
		for {
			e.processQueue()
			time.Sleep(3 * time.Second)
		}
	}()
}

func (e *IntegrationEngine) processQueue() {
	now := time.Now().Format(time.RFC3339)
	rows, err := db.Query("SELECT id, route_id, payload, retry_count FROM queue_messages WHERE status IN ('pending', 'failed') AND next_retry_at <= ?", now)
	if err != nil {
		return
	}
	defer rows.Close()

	var messages []QueueMessage
	for rows.Next() {
		var msg QueueMessage
		rows.Scan(&msg.ID, &msg.RouteID, &msg.Payload, &msg.RetryCount)
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		log.Println("Queue worker rows error:", err)
	}

	for _, msg := range messages {
		db.Exec("UPDATE queue_messages SET status = 'processing' WHERE id = ?", msg.ID)

		route, exists := e.Routes[msg.RouteID]
		if !exists || !route.Active {
			db.Exec("UPDATE queue_messages SET status = 'pending' WHERE id = ?", msg.ID)
			continue
		}

		success, _, _ := e.ExecuteRoute(msg.RouteID, msg.Payload)
		if success {
			db.Exec("UPDATE queue_messages SET status = 'success' WHERE id = ?", msg.ID)
		} else {
			newCount := msg.RetryCount + 1
			if newCount >= MaxRetries {
				db.Exec("UPDATE queue_messages SET status = 'dead_letter', retry_count = ? WHERE id = ?", newCount, msg.ID)
			} else {
				delay := time.Duration(1<<msg.RetryCount) * BaseDelay
				nextRetry := time.Now().Add(delay).Format(time.RFC3339)
				db.Exec("UPDATE queue_messages SET status = 'failed', retry_count = ?, next_retry_at = ? WHERE id = ?", newCount, nextRetry, msg.ID)
			}
		}
	}
}

// ----------------------------------------------------------------------------
// POLLER MANAGER
// ----------------------------------------------------------------------------

func (e *IntegrationEngine) StartPollers() {
	// Stop existing pollers
	for _, ticker := range e.Pollers {
		ticker.Stop()
	}
	e.Pollers = make(map[string]*time.Ticker)

	for _, route := range e.Routes {
		if !route.Active {
			continue
		}
		source := e.GetEndpoint(route, "source")
		if source == nil {
			continue
		}

		var cfg map[string]interface{}
		json.Unmarshal([]byte(source.Config), &cfg)

		switch source.Protocol {
		case "http-poll":
			e.startHTTPPoller(route.ID, route, cfg)
		case "database":
			e.startDBPoller(route.ID, route, cfg)
		}
	}
}

func (e *IntegrationEngine) startHTTPPoller(routeID string, route Route, cfg map[string]interface{}) {
	intervalStr, _ := cfg["pollInterval"].(string)
	if intervalStr == "" {
		intervalStr = "60"
	}
	intervalDuration, err := time.ParseDuration(intervalStr + "s")
	if err != nil || intervalDuration < time.Second {
		intervalDuration = 60 * time.Second
	}

	url, _ := cfg["url"].(string)
	if url == "" {
		return
	}

	ticker := time.NewTicker(intervalDuration)
	e.Pollers[routeID] = ticker

	go func() {
		// Fire immediately on start
		e.executeHTTPPoll(routeID, route, cfg)
		for range ticker.C {
			e.executeHTTPPoll(routeID, route, cfg)
		}
	}()
	log.Printf("Started HTTP Poller for route %s every %s", routeID, intervalDuration)
}

func (e *IntegrationEngine) executeHTTPPoll(routeID string, route Route, cfg map[string]interface{}) {
	url, _ := cfg["url"].(string)
	url = e.interpolateVariables(url, route)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[Poller %s] Request error: %v", routeID, err)
		return
	}

	if authType, ok := cfg["authType"].(string); ok {
		switch authType {
		case "basic":
			user, _ := cfg["username"].(string)
			pass, _ := cfg["password"].(string)
			req.SetBasicAuth(e.interpolateVariables(user, route), e.interpolateVariables(pass, route))
		case "bearer":
			token, _ := cfg["token"].(string)
			req.Header.Set("Authorization", "Bearer "+e.interpolateVariables(token, route))
		}
	}

	if headersRaw, ok := cfg["headers"].(string); ok && headersRaw != "" {
		headersRaw = e.interpolateVariables(headersRaw, route)
		var hdrs map[string]string
		if err := json.Unmarshal([]byte(headersRaw), &hdrs); err == nil {
			for k, v := range hdrs {
				req.Header.Set(k, v)
			}
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Poller %s] Fetch error: %v", routeID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		if len(bodyBytes) > 0 {
			// Enqueue the fetched data to be processed!
			e.EnqueueMessage(routeID, string(bodyBytes))
		}
	}
}

func (e *IntegrationEngine) startDBPoller(routeID string, route Route, cfg map[string]interface{}) {
	intervalStr, _ := cfg["pollInterval"].(string)
	if intervalStr == "" {
		intervalStr = "60"
	}
	intervalDuration, err := time.ParseDuration(intervalStr + "s")
	if err != nil || intervalDuration < time.Second {
		intervalDuration = 60 * time.Second
	}

	query, _ := cfg["query"].(string)
	if query == "" {
		return
	}

	ticker := time.NewTicker(intervalDuration)
	e.Pollers[routeID] = ticker

	go func() {
		// Fire immediately on start
		e.executeDBPoll(routeID, route, cfg)
		for range ticker.C {
			e.executeDBPoll(routeID, route, cfg)
		}
	}()
	log.Printf("Started DB Poller for route %s every %s", routeID, intervalDuration)
}

func (e *IntegrationEngine) executeDBPoll(routeID string, route Route, cfg map[string]interface{}) {
	driver, _ := cfg["driver"].(string)
	switch driver {
	case "", "mariadb":
		driver = "mysql"
	case "iris":
		driver = "odbc"
	}
	dsn, _ := cfg["dsn"].(string)
	dsn = e.interpolateVariables(dsn, route)

	query, _ := cfg["query"].(string)
	query = e.interpolateVariables(query, route)

	postQuery, _ := cfg["postQuery"].(string)
	postQuery = e.interpolateVariables(postQuery, route)

	if dsn == "" || query == "" {
		return
	}

	pollDb, err := sql.Open(driver, dsn)
	if err != nil {
		log.Printf("[DB Poller %s] Connect error: %v", routeID, err)
		return
	}
	defer pollDb.Close()

	rows, err := pollDb.Query(query)
	if err != nil {
		log.Printf("[DB Poller %s] Query error: %v", routeID, err)
		return
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return
	}

	enqueuedCount := e.processDBRows(routeID, rows, cols)

	if err := rows.Err(); err != nil {
		log.Printf("[DB Poller %s] Rows iteration error: %v", routeID, err)
	}

	if enqueuedCount > 0 && postQuery != "" {
		_, err = pollDb.Exec(postQuery)
		if err != nil {
			log.Printf("[DB Poller %s] Post-Query error: %v", routeID, err)
		} else {
			log.Printf("[DB Poller %s] Processed %d rows and executed post-query", routeID, enqueuedCount)
		}
	} else if enqueuedCount > 0 {
		log.Printf("[DB Poller %s] Processed %d rows", routeID, enqueuedCount)
	}
}

// ----------------------------------------------------------------------------
// PRUNE MANAGER
// ----------------------------------------------------------------------------

func (e *IntegrationEngine) StartPruner() {
	go func() {
		for {
			time.Sleep(24 * time.Hour)
			
			// Prune logs older than 30 days
			pruneDate := time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339)
			
			db.Exec("DELETE FROM message_logs WHERE created_at < ?", pruneDate)
			db.Exec("DELETE FROM queue_messages WHERE created_at < ? AND status IN ('success', 'dead_letter')", pruneDate)
			
			log.Println("Pruned old message logs and queue_messages")
		}

	}()
}

func (e *IntegrationEngine) processDBRows(routeID string, rows *sql.Rows, cols []string) int {
	enqueuedCount := 0
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}
		if err := rows.Scan(columnPointers...); err != nil {
			log.Printf("[DB Poller %s] Row scan error: %v", routeID, err)
			continue
		}
		rowData := parseDBRow(cols, columnPointers)
		jsonBytes, err := json.Marshal(rowData)
		if err == nil {
			e.EnqueueMessage(routeID, string(jsonBytes))
			enqueuedCount++
		}
	}
	return enqueuedCount
}

func parseDBRow(cols []string, columnPointers []interface{}) map[string]interface{} {
	rowData := make(map[string]interface{})
	for i, colName := range cols {
		val := columnPointers[i].(*interface{})
		if *val != nil {
			switch v := (*val).(type) {
			case []byte:
				rowData[colName] = string(v)
			default:
				rowData[colName] = v
			}
		} else {
			rowData[colName] = nil
		}
	}
	return rowData
}
