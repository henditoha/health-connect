package main

import (
	"encoding/json"
	"io"
	"log"
	"time"
	"net"
	"database/sql"
	"strconv"
	"os"
	"os/exec"
	"fmt"
	"embed"
	"io/fs"
	"strings"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	errInvalidRequest       = "Invalid request"
	maskedSecretPlaceholder = "********"
)

//go:embed dist/*
var frontendFS embed.FS

var engine *IntegrationEngine

type AppConfig struct {
	Port int `json:"port"`
}

var Config AppConfig

func loadConfig() {
	Config = AppConfig{
		Port: 3001,
	}
	file, err := os.ReadFile("config.json")
	if err == nil {
		json.Unmarshal(file, &Config)
	} else {
		bytes, _ := json.MarshalIndent(Config, "", "  ")
		os.WriteFile("config.json", bytes, 0644)
	}
}

func main() {
	loadConfig()
	gin.SetMode(gin.ReleaseMode)

	// Check if already running
	url := fmt.Sprintf("http://localhost:%d", Config.Port)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", Config.Port), 1*time.Second)
	if err == nil {
		// Already running, just open browser and exit
		conn.Close()
		openBrowser(url)
		os.Exit(0)
	}

	err = initDb()
	if err != nil {
		panic(err)
	}

	InitLicenseSystem()

	engine = NewIntegrationEngine()
	engine.LoadActiveRoutes()
	engine.StartQueueWorker()
	engine.StartPruner()

	r := gin.Default()
	r.Use(cors.Default())

	// Management APIs
	api := r.Group("/api")
	{
		api.GET("/groups", getGroups)
		api.POST("/groups", createGroup)
		api.GET("/routes", getRoutes)
		api.POST("/routes", createRoute)
		api.PUT("/routes/:id", updateRoute)
		api.GET("/logs", getLogs)
		api.GET("/stats", getStats)
		api.POST("/test-connection", testConnection)
		api.PUT("/routes/:id/toggle", toggleRoute)
		api.POST("/routes/:id/clone", cloneRoute)
		api.DELETE("/routes/:id", deleteRoute)
		
		api.GET("/variables", getVariables)
		api.POST("/variables", createVariable)
		api.PUT("/variables/:id", updateVariable)
		api.DELETE("/variables/:id", deleteVariable)

		api.GET("/license", getLicenseInfo)
		api.POST("/license/activate", activateLicense)

		api.POST("/login", login)
		api.GET("/users", getUsers)
		api.POST("/users", createUser)
		api.PUT("/users/:id", updateUser)
		api.PUT("/users/:id/status", toggleUserStatus)
	}

	// Ingress
	r.POST("/ingress/:routeId", handleIngress)

	// Serve Frontend
	subFS, err := fs.Sub(frontendFS, "dist")
	if err == nil {
		httpFS := http.FS(subFS)
		fileServer := http.FileServer(httpFS)
		
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api/") {
				c.JSON(404, gin.H{"error": "Endpoint not found"})
				return
			}
			
			_, err := subFS.Open(strings.TrimPrefix(path, "/"))
			if err != nil {
				c.Request.URL.Path = "/"
			}
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser(url)
	}()

	r.Run(fmt.Sprintf(":%d", Config.Port))
}

func openBrowser(url string) {
	exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

func getGroups(c *gin.Context) {
	rows, err := db.Query("SELECT id, name, description FROM route_groups ORDER BY name ASC")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var groups []RouteGroup
	for rows.Next() {
		var g RouteGroup
		var desc sql.NullString
		rows.Scan(&g.ID, &g.Name, &desc)
		g.Description = desc.String
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if groups == nil { groups = make([]RouteGroup, 0) }
	c.JSON(200, groups)
}

func createGroup(c *gin.Context) {
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	id := uuid.New().String()
	_, err := db.Exec("INSERT INTO route_groups (id, name, description) VALUES (?, ?, ?)", id, input.Name, input.Description)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"id": id, "message": "Group created"})
}

func getRoutes(c *gin.Context) {
	routes := make([]Route, 0)
	for _, r := range engine.Routes {
		routes = append(routes, r)
	}
	c.JSON(200, routes)
}

func createRoute(c *gin.Context) {
	if err := CheckCanCreateRoute(); err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}

	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		GroupName   string `json:"group_name"`
	}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	id := uuid.New().String()
	db.Exec("INSERT INTO routes (id, name, description, group_name, active) VALUES (?, ?, ?, ?, 1)", id, input.Name, input.Description, input.GroupName)

	sid := uuid.New().String()
	db.Exec("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, 'source', 'http', 'json', ?)", sid, id, `{"path":"/ingress/`+id+`"}`)

	tid := uuid.New().String()
	db.Exec("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, 'target', 'http', 'json', ?)", tid, id, `{"url":"http://example.com/api"}`)

	trid := uuid.New().String()
	db.Exec("INSERT INTO transformers (id, route_id, source_script, script, inbound_template, outbound_template) VALUES (?, ?, ?, ?, ?, ?)", trid, id, "", `let data = JSON.parse(input); return JSON.stringify(data);`, `{}`, `{}`)

	engine.LoadActiveRoutes()
	c.JSON(200, gin.H{"id": id, "message": "Created"})
}

func toggleRoute(c *gin.Context) {
	id := c.Param("id")
	r, exists := engine.Routes[id]
	if !exists {
		c.JSON(404, gin.H{"error": "Route not found"})
		return
	}

	newStatus := 0
	if !r.Active {
		if err := CheckMaxRouteLimit(); err != nil {
			c.JSON(403, gin.H{"error": err.Error()})
			return
		}
		newStatus = 1
	}

	_, err := db.Exec("UPDATE routes SET active = ? WHERE id = ?", newStatus, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	engine.LoadActiveRoutes()
	c.JSON(200, gin.H{"message": "Route status toggled", "active": newStatus == 1})
}

func cloneRoute(c *gin.Context) {
	if err := CheckCanCreateRoute(); err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}

	id := c.Param("id")
	r, exists := engine.Routes[id]
	if !exists {
		c.JSON(404, gin.H{"error": "Route not found"})
		return
	}

	newId := uuid.New().String()
	newName := r.Name + " (Copy)"
	
	varsJSON, _ := json.Marshal(r.Variables)
	
	_, err := db.Exec("INSERT INTO routes (id, name, description, group_name, active, variables) VALUES (?, ?, ?, ?, 0, ?)", newId, newName, r.Description, r.GroupName, string(varsJSON))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	for _, ep := range r.Endpoints {
		epId := uuid.New().String()
		db.Exec("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, ?, ?, ?, ?)", epId, newId, ep.Role, ep.Protocol, ep.Format, ep.Config)
	}

	transId := uuid.New().String()
	db.Exec("INSERT INTO transformers (id, route_id, source_script, script, inbound_template, outbound_template) VALUES (?, ?, ?, ?, ?, ?)", transId, newId, r.Transformer.SourceScript, r.Transformer.Script, r.Transformer.InboundTemplate, r.Transformer.OutboundTemplate)

	engine.LoadActiveRoutes()
	c.JSON(200, gin.H{"id": newId, "message": "Route cloned successfully"})
}

func deleteRoute(c *gin.Context) {
	if err := CheckCanDeleteRoute(); err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}

	id := c.Param("id")
	db.Exec("DELETE FROM endpoints WHERE route_id=?", id)
	db.Exec("DELETE FROM transformers WHERE route_id=?", id)
	db.Exec("DELETE FROM routes WHERE id=?", id)

	engine.LoadActiveRoutes()
	c.JSON(200, gin.H{"message": "Route deleted successfully"})
}

func updateRoute(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		GroupName   string `json:"group_name"`
		Source struct {
			Protocol string                 `json:"protocol"`
			Format   string                 `json:"format"`
			Config   map[string]interface{} `json:"config"`
		} `json:"source"`
		Target struct {
			Protocol string                 `json:"protocol"`
			Format   string                 `json:"format"`
			Config   map[string]interface{} `json:"config"`
		} `json:"target"`
		Targets []struct {
			ID       string                 `json:"id"`
			Protocol string                 `json:"protocol"`
			Format   string                 `json:"format"`
			Config   map[string]interface{} `json:"config"`
		} `json:"targets"`
		Transformer Transformer       `json:"transformer"`
		Variables   map[string]string `json:"variables"`
	}
	
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	varsJSON, _ := json.Marshal(input.Variables)
	db.Exec("UPDATE routes SET name=?, description=?, group_name=?, variables=? WHERE id=?", input.Name, input.Description, input.GroupName, string(varsJSON), id)

	scfg, _ := json.Marshal(input.Source.Config)
	db.Exec("UPDATE endpoints SET protocol=?, format=?, config=? WHERE route_id=? AND role='source'", input.Source.Protocol, input.Source.Format, string(scfg), id)

	if len(input.Targets) > 0 {
		// Multi-destination target updates
		db.Exec("DELETE FROM endpoints WHERE route_id=? AND role='target'", id)
		for _, tgt := range input.Targets {
			tid := tgt.ID
			if tid == "" { tid = uuid.New().String() }
			tcfg, _ := json.Marshal(tgt.Config)
			proto := tgt.Protocol
			if proto == "" { proto = "http" }
			fmt := tgt.Format
			if fmt == "" { fmt = "json" }
			db.Exec("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, 'target', ?, ?, ?)",
				tid, id, proto, fmt, string(tcfg))
		}
	} else if input.Target.Protocol != "" {
		tcfg, _ := json.Marshal(input.Target.Config)
		db.Exec("UPDATE endpoints SET protocol=?, format=?, config=? WHERE route_id=? AND role='target'", input.Target.Protocol, input.Target.Format, string(tcfg), id)
	}

	db.Exec("UPDATE transformers SET source_script=?, script=?, inbound_template=?, outbound_template=? WHERE route_id=?", input.Transformer.SourceScript, input.Transformer.Script, input.Transformer.InboundTemplate, input.Transformer.OutboundTemplate, id)

	engine.LoadActiveRoutes()
	c.JSON(200, gin.H{"message": "Updated"})
}

func findRouteByIDOrName(target string) (*Route, bool) {
	if r, exists := engine.Routes[target]; exists {
		return &r, true
	}
	targetSlug := strings.ToLower(strings.ReplaceAll(target, " ", "-"))
	for _, r := range engine.Routes {
		rSlug := strings.ToLower(strings.ReplaceAll(r.Name, " ", "-"))
		if strings.EqualFold(r.ID, target) || strings.EqualFold(r.Name, target) || rSlug == targetSlug {
			return &r, true
		}
	}
	return nil, false
}

func getRouteAckBehavior(route Route) string {
	source := engine.GetEndpoint(route, "source")
	if source == nil {
		return "deferred"
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(source.Config), &cfg); err == nil {
		if ab, ok := cfg["ackBehavior"].(string); ok && ab != "" {
			return ab
		}
	}
	return "deferred"
}

func handleIngress(c *gin.Context) {
	routeId := c.Param("routeId")
	body, _ := io.ReadAll(c.Request.Body)

	route, exists := findRouteByIDOrName(routeId)
	if !exists {
		c.JSON(404, gin.H{"error": "Channel endpoint not found"})
		return
	}

	if getRouteAckBehavior(*route) == "immediate" {
		c.String(200, "ACK")
		engine.EnqueueMessage(route.ID, string(body))
		return
	}

	success, resp, err := engine.ExecuteRoute(route.ID, string(body))
	if success {
		c.String(200, resp)
	} else {
		c.JSON(500, gin.H{"error": err.Error()})
	}
}

func getLogs(c *gin.Context) {
	search := c.Query("search")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	channel := c.Query("channel")
	
	pageStr := c.Query("page")
	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	
	limitStr := c.Query("limit")
	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	
	offset := (page - 1) * limit

	query := `SELECT m.id, m.route_id, r.name, m.source_protocol, m.target_protocol, m.inbound_data, m.outbound_data, m.response_data, m.status, m.error_message, m.created_at 
	FROM message_logs m LEFT JOIN routes r ON m.route_id = r.id WHERE 1=1`
	
	countQuery := `SELECT COUNT(*) FROM message_logs m LEFT JOIN routes r ON m.route_id = r.id WHERE 1=1`
	
	var args []interface{}

	if search != "" {
		searchParam := "%" + search + "%"
		query += ` AND (m.inbound_data LIKE ? OR m.outbound_data LIKE ? OR m.response_data LIKE ? OR m.error_message LIKE ?)`
		countQuery += ` AND (m.inbound_data LIKE ? OR m.outbound_data LIKE ? OR m.response_data LIKE ? OR m.error_message LIKE ?)`
		args = append(args, searchParam, searchParam, searchParam, searchParam)
	}
	if startDate != "" {
		query += ` AND m.created_at >= ?`
		countQuery += ` AND m.created_at >= ?`
		args = append(args, startDate)
	}
	if endDate != "" {
		query += ` AND m.created_at <= ?`
		countQuery += ` AND m.created_at <= ?`
		args = append(args, endDate)
	}
	if channel != "" {
		query += ` AND r.name = ?`
		countQuery += ` AND r.name = ?`
		args = append(args, channel)
	}

	query += ` ORDER BY m.created_at DESC LIMIT ? OFFSET ?`
	
	var total int
	err := db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to count logs: " + err.Error()})
		return
	}

	args = append(args, limit, offset)
	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch logs: " + err.Error()})
		return
	}
	defer rows.Close()

	logs, err := scanLogRows(rows)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to fetch logs: " + err.Error()})
		return
	}
	
	if logs == nil {
		logs = []MessageLog{}
	}

	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	c.JSON(200, gin.H{
		"logs": logs,
		"total": total,
		"page": page,
		"totalPages": totalPages,
	})
}

func scanLogRows(rows *sql.Rows) ([]MessageLog, error) {
	var logs []MessageLog
	for rows.Next() {
		var l MessageLog
		var rId, rn, src, tgt, in, out, res, st, errM, cr sql.NullString
		err := rows.Scan(&l.ID, &rId, &rn, &src, &tgt, &in, &out, &res, &st, &errM, &cr)
		if err != nil {
			log.Println("Error scanning log row:", err)
			continue
		}
		l.RouteID = rId.String
		l.RouteName = rn.String
		l.SourceProtocol = src.String
		l.TargetProtocol = tgt.String
		l.InboundData = in.String
		l.OutboundData = out.String
		l.ResponseData = res.String
		l.Status = st.String
		l.ErrorMessage = errM.String
		l.CreatedAt = cr.String
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func testConnection(c *gin.Context) {
	var input struct {
		Role           string                 `json:"role"`
		Protocol       string                 `json:"protocol"`
		Config         map[string]interface{} `json:"config"`
		LocalVariables interface{}            `json:"local_variables"`
	}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": "Invalid input"})
		return
	}

	if input.Role == "target" {
		testTargetConnection(c, input.Protocol, input.Config, input.LocalVariables)
	} else {
		testSourceConnection(c, input.Protocol, input.Config, input.LocalVariables)
	}
}

func testTargetConnection(c *gin.Context, protocol string, config map[string]interface{}, localVarsRaw interface{}) {
	dummyRoute := Route{}
	if raw, ok := localVarsRaw.(string); ok && raw != "" {
		json.Unmarshal([]byte(raw), &dummyRoute.Variables)
	} else if m, ok := localVarsRaw.(map[string]interface{}); ok {
		dummyRoute.Variables = make(map[string]string)
		for k, v := range m {
			dummyRoute.Variables[k] = fmt.Sprint(v)
		}
	}

	switch protocol {
	case "tcp":
		host, _ := config["host"].(string)
		port, _ := config["port"].(string)
		host = engine.interpolateVariables(host, dummyRoute)
		port = engine.interpolateVariables(port, dummyRoute)
		if host == "" { host = "127.0.0.1" }
		if port == "" { port = "80" }
		conn, err := net.DialTimeout("tcp", host+":"+port, 4*time.Second)
		if err != nil {
			c.JSON(400, gin.H{"error": "TCP Dial Failed: " + err.Error()})
			return
		}
		conn.Close()
		c.JSON(200, gin.H{"success": true, "message": fmt.Sprintf("TCP Connection to %s:%s successful", host, port)})

	case "http":
		testTargetHTTPConnection(c, config, dummyRoute)

	case "database":
		testDatabaseConnection(c, config, localVarsRaw)

	case "internal-channel":
		routeId, _ := config["route_id"].(string)
		if routeId == "" {
			c.JSON(400, gin.H{"error": "Please select a destination channel"})
			return
		}
		if _, exists := engine.Routes[routeId]; !exists {
			c.JSON(400, gin.H{"error": "Destination channel not found"})
			return
		}
		c.JSON(200, gin.H{"success": true, "message": "Destination channel exists and is ready"})

	default:
		c.JSON(200, gin.H{"success": true, "message": "Target endpoint ready for deployment"})
	}
}

func buildTargetURL(config map[string]interface{}) string {
	if u, ok := config["url"].(string); ok && u != "" {
		return u
	}
	host, _ := config["host"].(string)
	if host == "" {
		host = "localhost"
	}
	port, _ := config["port"].(string)
	path, _ := config["path"].(string)

	scheme := "http://"
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		scheme = ""
	}

	portStr := ""
	if port != "" && !strings.Contains(host, ":"+port) {
		portStr = ":" + port
	}

	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return scheme + host + portStr + path
}

func applyTargetHeaders(req *http.Request, config map[string]interface{}, dummyRoute Route) {
	headersRaw, ok := config["headers"].(string)
	if !ok || headersRaw == "" {
		return
	}
	headersRaw = engine.interpolateVariables(headersRaw, dummyRoute)
	var hdrs map[string]string
	if err := json.Unmarshal([]byte(headersRaw), &hdrs); err == nil {
		for k, v := range hdrs {
			req.Header.Set(k, v)
		}
	}
}

func applyTargetAuth(req *http.Request, config map[string]interface{}, dummyRoute Route) {
	authType, ok := config["authType"].(string)
	if !ok {
		return
	}
	switch authType {
	case "basic":
		user, _ := config["username"].(string)
		pass, _ := config["password"].(string)
		req.SetBasicAuth(engine.interpolateVariables(user, dummyRoute), engine.interpolateVariables(pass, dummyRoute))
	case "bearer":
		token, _ := config["token"].(string)
		req.Header.Set("Authorization", "Bearer "+engine.interpolateVariables(token, dummyRoute))
	}
}

func doHTTPPing(client *http.Client, req *http.Request) (*http.Response, error) {
	resp, err := client.Do(req)
	if err == nil && (resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented) {
		resp.Body.Close()
		req.Method = "GET"
		resp, err = client.Do(req)
	}
	return resp, err
}

func testTargetHTTPConnection(c *gin.Context, config map[string]interface{}, dummyRoute Route) {
	rawURL := buildTargetURL(config)
	urlStr := engine.interpolateVariables(rawURL, dummyRoute)

	req, err := http.NewRequest("HEAD", urlStr, nil)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid URL: " + err.Error()})
		return
	}

	applyTargetHeaders(req, config, dummyRoute)
	applyTargetAuth(req, config, dummyRoute)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := doHTTPPing(client, req)
	if err != nil {
		c.JSON(400, gin.H{"error": "Target connection failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		c.JSON(200, gin.H{"success": true, "message": fmt.Sprintf("Connected to target! HTTP %d %s", resp.StatusCode, resp.Status)})
	} else {
		// Server responded with 4xx or 5xx, meaning host is reachable!
		c.JSON(200, gin.H{"success": true, "message": fmt.Sprintf("Target reached! Status: HTTP %d %s", resp.StatusCode, resp.Status)})
	}
}


func testSourceConnection(c *gin.Context, protocol string, config map[string]interface{}, localVarsRaw interface{}) {
	dummyRoute := Route{}
	if raw, ok := localVarsRaw.(string); ok && raw != "" {
		json.Unmarshal([]byte(raw), &dummyRoute.Variables)
	} else if m, ok := localVarsRaw.(map[string]interface{}); ok {
		dummyRoute.Variables = make(map[string]string)
		for k, v := range m { dummyRoute.Variables[k] = fmt.Sprint(v) }
	}

	switch protocol {
	case "database":
		testDatabaseConnection(c, config, localVarsRaw)
	case "http-poll":
		testHTTPPollConnection(c, config, localVarsRaw)
	case "tcp":
		port, _ := config["port"].(string)
		port = engine.interpolateVariables(port, dummyRoute)
		if strings.TrimSpace(port) == "" {
			c.JSON(400, gin.H{"error": "TCP Listener port is required"})
			return
		}
		c.JSON(200, gin.H{"success": true, "message": fmt.Sprintf("TCP Listener Port :%s is configured and ready", port)})
	case "http":
		path, _ := config["path"].(string)
		path = engine.interpolateVariables(path, dummyRoute)
		if strings.TrimSpace(path) == "" {
			path = "/ingress/:id"
		}
		c.JSON(200, gin.H{"success": true, "message": fmt.Sprintf("HTTP Ingress endpoint '%s' is valid", path)})
	default:
		c.JSON(200, gin.H{"success": true, "message": "Source connector configured successfully"})
	}
}

func testDatabaseConnection(c *gin.Context, config map[string]interface{}, localVarsRaw interface{}) {
	driver, _ := config["driver"].(string)
	dsn, _ := config["dsn"].(string)
	
	switch driver {
	case "mariadb":
		driver = "mysql"
	case "iris":
		driver = "odbc"
	}
	
	if driver == "" || dsn == "" {
		c.JSON(400, gin.H{"error": "Driver or DSN is empty"})
		return
	}
	
	dummyRoute := Route{}
	if raw, ok := localVarsRaw.(string); ok && raw != "" {
		json.Unmarshal([]byte(raw), &dummyRoute.Variables)
	} else if m, ok := localVarsRaw.(map[string]interface{}); ok {
		dummyRoute.Variables = make(map[string]string)
		for k, v := range m { dummyRoute.Variables[k] = fmt.Sprint(v) }
	}

	dsn = engine.interpolateVariables(dsn, dummyRoute)
	
	testDb, err := sql.Open(driver, dsn)
	if err != nil {
		c.JSON(400, gin.H{"error": "SQL Open Error: " + err.Error()})
		return
	}
	defer testDb.Close()
	
	err = testDb.Ping()
	if err != nil {
		c.JSON(400, gin.H{"error": "DB Ping Failed: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "message": "Database Connection Successful!"})
}

func testHTTPPollConnection(c *gin.Context, config map[string]interface{}, localVarsRaw interface{}) {
	urlStr, _ := config["url"].(string)
	headersRaw, _ := config["headers"].(string)

	if urlStr == "" {
		c.JSON(400, gin.H{"error": "API URL cannot be empty"})
		return
	}

	dummyRoute := Route{}
	if raw, ok := localVarsRaw.(string); ok && raw != "" {
		json.Unmarshal([]byte(raw), &dummyRoute.Variables)
	} else if m, ok := localVarsRaw.(map[string]interface{}); ok {
		dummyRoute.Variables = make(map[string]string)
		for k, v := range m { dummyRoute.Variables[k] = fmt.Sprint(v) }
	}
	
	urlStr = engine.interpolateVariables(urlStr, dummyRoute)
	headersRaw = engine.interpolateVariables(headersRaw, dummyRoute)

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid URL format: " + err.Error()})
		return
	}

	if headersRaw != "" {
		var hdrs map[string]string
		if err := json.Unmarshal([]byte(headersRaw), &hdrs); err == nil {
			for k, v := range hdrs {
				req.Header.Set(k, v)
			}
		} else {
			c.JSON(400, gin.H{"error": "Invalid JSON format in Custom Headers"})
			return
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(400, gin.H{"error": "HTTP GET Failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		c.JSON(400, gin.H{"error": fmt.Sprintf("API returned error status: %d %s", resp.StatusCode, resp.Status)})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": fmt.Sprintf("HTTP GET Successful (Status %d)", resp.StatusCode)})
}

// User Management Handler Implementations

func login(c *gin.Context) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": errInvalidRequest})
		return
	}

	var hashed string
	var active bool
	var user User
	err := db.QueryRow("SELECT id, username, password_hash, active FROM users WHERE username = ?", input.Username).Scan(&user.ID, &user.Username, &hashed, &active)
	if err != nil {
		c.JSON(401, gin.H{"error": "Invalid credentials"})
		return
	}
	if !active {
		c.JSON(401, gin.H{"error": "User is disabled"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(input.Password)); err != nil {
		c.JSON(401, gin.H{"error": "Invalid credentials"})
		return
	}

	c.JSON(200, gin.H{"success": true, "user": user, "token": user.ID}) // simplified token auth
}

func getUsers(c *gin.Context) {
	rows, err := db.Query("SELECT id, username, active FROM users ORDER BY username ASC")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Username, &u.Active)
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if users == nil { users = make([]User, 0) }
	c.JSON(200, users)
}

func createUser(c *gin.Context) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": errInvalidRequest})
		return
	}
	
	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(500, gin.H{"error": "Encryption error"})
		return
	}

	id := uuid.New().String()
	_, err = db.Exec("INSERT INTO users (id, username, password_hash, active) VALUES (?, ?, ?, ?)", id, input.Username, string(hashed), true)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create user (might already exist)"})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func updateUser(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Password string `json:"password"`
	}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": errInvalidRequest})
		return
	}
	
	hashed, _ := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	_, err := db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(hashed), id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

func toggleUserStatus(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Active bool `json:"active"`
	}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(400, gin.H{"error": errInvalidRequest})
		return
	}
	
	_, err := db.Exec("UPDATE users SET active = ? WHERE id = ?", input.Active, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true})
}

// Variable Handlers
func getVariables(c *gin.Context) {
	rows, err := db.Query("SELECT id, key_name, value_data, is_secret, description FROM global_variables ORDER BY key_name ASC")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var vars []GlobalVariable
	for rows.Next() {
		var v GlobalVariable
		var desc sql.NullString
		rows.Scan(&v.ID, &v.KeyName, &v.ValueData, &v.IsSecret, &desc)
		if desc.Valid {
			v.Description = desc.String
		}
		if v.IsSecret {
			v.ValueData = maskedSecretPlaceholder // Mask secret on the frontend
		}
		vars = append(vars, v)
	}
	if err := rows.Err(); err != nil {
		log.Println("Error iterating rows in getVariables:", err)
	}

	if vars == nil {
		vars = []GlobalVariable{}
	}

	c.JSON(200, vars)
}

func createVariable(c *gin.Context) {
	var v GlobalVariable
	if err := c.BindJSON(&v); err != nil {
		c.JSON(400, gin.H{"error": errInvalidRequest})
		return
	}

	valToSave := v.ValueData
	if v.IsSecret {
		encrypted, err := EncryptSecret(v.ValueData)
		if err == nil {
			valToSave = encrypted
		}
	}

	v.ID = uuid.New().String()
	_, err := db.Exec(queryInsertGlobalVar,
		v.ID, v.KeyName, valToSave, v.IsSecret, v.Description)
	
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create variable (maybe duplicate key?)"})
		return
	}
	engine.loadGlobalVariables()
	if v.IsSecret {
		v.ValueData = maskedSecretPlaceholder
	}
	c.JSON(200, v)
}

func updateVariable(c *gin.Context) {
	id := c.Param("id")
	var v GlobalVariable
	if err := c.BindJSON(&v); err != nil {
		c.JSON(400, gin.H{"error": errInvalidRequest})
		return
	}

	// If it's a secret and the frontend sent the masked value, don't update the value_data
	if v.IsSecret && v.ValueData == maskedSecretPlaceholder {
		_, err := db.Exec("UPDATE global_variables SET key_name=?, is_secret=?, description=? WHERE id=?",
			v.KeyName, v.IsSecret, v.Description, id)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	} else {
		valToSave := v.ValueData
		if v.IsSecret {
			encrypted, err := EncryptSecret(v.ValueData)
			if err == nil {
				valToSave = encrypted
			}
		}
		_, err := db.Exec("UPDATE global_variables SET key_name=?, value_data=?, is_secret=?, description=? WHERE id=?",
			v.KeyName, valToSave, v.IsSecret, v.Description, id)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
	}
	engine.loadGlobalVariables()
	c.JSON(200, gin.H{"status": "success"})
}

func deleteVariable(c *gin.Context) {
	id := c.Param("id")
	_, err := db.Exec("DELETE FROM global_variables WHERE id=?", id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	engine.loadGlobalVariables()
	c.JSON(200, gin.H{"status": "success"})
}
