package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/google/uuid"
)

type IntegrationEngine struct {
	Routes        map[string]Route
	TCPServers    map[string]net.Listener
	Pollers       map[string]*time.Ticker
	GlobalVars    map[string]string
}

func NewIntegrationEngine() *IntegrationEngine {
	return &IntegrationEngine{
		Routes:     make(map[string]Route),
		TCPServers: make(map[string]net.Listener),
		Pollers:    make(map[string]*time.Ticker),
		GlobalVars: make(map[string]string),
	}
}

func (e *IntegrationEngine) LoadActiveRoutes() {
	e.loadGlobalVariables()
	
	rows, err := db.Query("SELECT id, name, description, group_name, active, variables FROM routes")
	if err != nil {
		log.Println("Error loading routes:", err)
		return
	}
	defer rows.Close()

	for _, srv := range e.TCPServers {
		srv.Close()
	}
	e.TCPServers = make(map[string]net.Listener)
	e.Routes = make(map[string]Route)

	for rows.Next() {
		r, err := scanRouteRow(rows)
		if err != nil {
			log.Println("Error scanning route:", err)
			continue
		}
		e.Routes[r.ID] = *r
		e.startTCPIfActive(*r)
	}
	if err := rows.Err(); err != nil {
		log.Println("Error iterating rows:", err)
	}
	e.StartPollers()
	log.Printf("Loaded %d routes into Go Engine", len(e.Routes))
}

func scanRouteRow(rows *sql.Rows) (*Route, error) {
	var r Route
	var desc, grp, varsJSON sql.NullString
	var active sql.NullInt64
	err := rows.Scan(&r.ID, &r.Name, &desc, &grp, &active, &varsJSON)
	if err != nil {
		return nil, err
	}
	r.Description = desc.String
	r.GroupName = grp.String
	r.Active = !active.Valid || active.Int64 == 1

	r.Variables = make(map[string]string)
	if varsJSON.Valid && varsJSON.String != "" {
		json.Unmarshal([]byte(varsJSON.String), &r.Variables)
	}

	loadEndpointsForRoute(&r)
	loadTransformerForRoute(&r)
	return &r, nil
}

func (e *IntegrationEngine) startTCPIfActive(r Route) {
	if !r.Active {
		return
	}
	source := e.GetEndpoint(r, "source")
	if source != nil && source.Protocol == "tcp" {
		e.StartTCPServer(r.ID, source.Config)
	}
}

func (e *IntegrationEngine) loadGlobalVariables() {
	rows, err := db.Query("SELECT key_name, value_data FROM global_variables")
	if err != nil {
		log.Println("Error loading global variables:", err)
		return
	}
	defer rows.Close()

	e.GlobalVars = make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			e.GlobalVars[k] = v
		}
	}
	if err := rows.Err(); err != nil {
		log.Println("Error iterating global variables:", err)
	}
}

func loadEndpointsForRoute(r *Route) {
	epRows, _ := db.Query("SELECT id, role, protocol, format, config FROM endpoints WHERE route_id = ?", r.ID)
	for epRows.Next() {
		var ep Endpoint
		var role, proto, format, config sql.NullString
		err := epRows.Scan(&ep.ID, &role, &proto, &format, &config)
		if err != nil {
			log.Println("Error scanning endpoint:", err)
			continue
		}
		ep.Role = role.String
		ep.Protocol = proto.String
		ep.Format = format.String
		ep.Config = config.String
		r.Endpoints = append(r.Endpoints, ep)
	}
	if err := epRows.Err(); err != nil {
		log.Println("Error iterating epRows:", err)
	}
	epRows.Close()
}

func loadTransformerForRoute(r *Route) {
	trRows, err := db.Query("SELECT id, source_script, script, inbound_template, outbound_template FROM transformers WHERE route_id = ?", r.ID)
	if err != nil {
		oldRows, _ := db.Query("SELECT id, script, inbound_template, outbound_template FROM transformers WHERE route_id = ?", r.ID)
		if oldRows != nil && oldRows.Next() {
			var script, inTpl, outTpl sql.NullString
			oldRows.Scan(&r.Transformer.ID, &script, &inTpl, &outTpl)
			r.Transformer.Script = script.String
			r.Transformer.InboundTemplate = inTpl.String
			r.Transformer.OutboundTemplate = outTpl.String
		}
		if oldRows != nil { oldRows.Close() }
		return
	}
	if trRows.Next() {
		var srcScript, script, inTpl, outTpl sql.NullString
		trRows.Scan(&r.Transformer.ID, &srcScript, &script, &inTpl, &outTpl)
		r.Transformer.SourceScript = srcScript.String
		r.Transformer.Script = script.String
		r.Transformer.InboundTemplate = inTpl.String
		r.Transformer.OutboundTemplate = outTpl.String
	}
	trRows.Close()
}

func runTransformerScript(script, inputData, inTpl, outTpl string) (string, error) {
	if script == "" {
		return inputData, nil
	}
	vm := goja.New()
	vm.Set("input", inputData)
	vm.Set("inboundTemplate", inTpl)
	vm.Set("outboundTemplate", outTpl)

	vm.Set("fetch", func(urlStr string) string {
		res, err := httpFetchHelper(urlStr)
		if err != nil {
			panic(vm.ToValue("Fetch error: " + err.Error()))
		}
		return res
	})

	vm.Set("dbQuery", func(driver string, dsn string, query string) string {
		res, err := dbQueryHelper(driver, dsn, query)
		if err != nil {
			panic(vm.ToValue("DB Query error: " + err.Error()))
		}
		return res
	})

	wrapped := fmt.Sprintf(`(function() { %s })()`, script)
	val, err := vm.RunString(wrapped)
	if err != nil {
		return "", err
	}
	return val.String(), nil
}

func httpFetchHelper(urlStr string) (string, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func dbQueryHelper(driver, dsn, query string) (string, error) {
	if driver == "iris" { driver = "odbc" }
	if driver == "mariadb" { driver = "mysql" }
	dbConn, err := sql.Open(driver, dsn)
	if err != nil {
		return "", err
	}
	defer dbConn.Close()

	rows, err := dbConn.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	result := scanQueryRows(rows, cols)
	if err := rows.Err(); err != nil {
		return "", err
	}
	b, err := json.Marshal(result)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func scanQueryRows(rows *sql.Rows, cols []string) []map[string]interface{} {
	var result []map[string]interface{}
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}
		if err := rows.Scan(columnPointers...); err != nil {
			continue
		}
		result = append(result, mapRowColumns(cols, columnPointers))
	}
	return result
}

func mapRowColumns(cols []string, columnPointers []interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for i, colName := range cols {
		val := columnPointers[i].(*interface{})
		if val == nil || *val == nil {
			continue
		}
		if bytesVal, ok := (*val).([]byte); ok {
			m[colName] = string(bytesVal)
		} else {
			m[colName] = *val
		}
	}
	return m
}

func (e *IntegrationEngine) GetEndpoint(r Route, role string) *Endpoint {
	for _, ep := range r.Endpoints {
		if ep.Role == role {
			return &ep
		}
	}
	return nil
}

func (e *IntegrationEngine) GetEndpointsByRole(r Route, role string) []Endpoint {
	var list []Endpoint
	for _, ep := range r.Endpoints {
		if ep.Role == role {
			list = append(list, ep)
		}
	}
	return list
}

func evalFilterScript(filterScript, inputData string) (bool, error) {
	trimmed := strings.TrimSpace(filterScript)
	if trimmed == "" {
		return true, nil
	}

	vm := goja.New()
	vm.Set("input", inputData)

	var parsedJSON interface{}
	if err := json.Unmarshal([]byte(inputData), &parsedJSON); err == nil {
		vm.Set("msg", parsedJSON)
	}

	scriptToRun := trimmed
	if strings.Contains(trimmed, "return") {
		scriptToRun = fmt.Sprintf(`(function() { %s })()`, trimmed)
	} else {
		scriptToRun = fmt.Sprintf(`(function() { return !!(%s); })()`, trimmed)
	}

	val, err := vm.RunString(scriptToRun)
	if err != nil {
		return false, fmt.Errorf("Filter evaluation error: %w", err)
	}

	return val.ToBoolean(), nil
}

func (e *IntegrationEngine) interpolateVariables(input string, route Route) string {
	result := input
	// Interpolate channel variables first
	for k, v := range route.Variables {
		placeholder := fmt.Sprintf("{{%s}}", k)
		result = strings.ReplaceAll(result, placeholder, v)
	}
	// Fallback to global variables
	for k, v := range e.GlobalVars {
		placeholder := fmt.Sprintf("{{%s}}", k)
		result = strings.ReplaceAll(result, placeholder, v)
	}
	return result
}

func (e *IntegrationEngine) StartTCPServer(routeID string, configJSON string) {
	var cfg map[string]interface{}
	json.Unmarshal([]byte(configJSON), &cfg)
	
	port, ok := cfg["port"].(string)
	if !ok || port == "" {
		return
	}

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Printf("Failed to start TCP source for route %s: %v", routeID, err)
		return
	}
	e.TCPServers[routeID] = listener
	log.Printf("Started TCP Listener for route %s on port %s", routeID, port)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go e.handleTCPConnection(routeID, conn, cfg)
		}
	}()
}

func (e *IntegrationEngine) handleTCPConnection(routeID string, conn net.Conn, cfg map[string]interface{}) {
	defer conn.Close()
	connOpened(routeID)
	defer connClosed(routeID)
	buf := make([]byte, 65536)
	
	ackBehavior := "deferred"
	if ab, ok := cfg["ackBehavior"].(string); ok {
		ackBehavior = ab
	}

	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		inputData := string(buf[:n])
		
		isMllp, parsedData := parseMLLP(inputData)

		sendResponse := func(msg string) {
			if isMllp {
				conn.Write(fmt.Appendf(nil, "\x0B%s\x1C\x0D", msg))
			} else {
				conn.Write([]byte(msg))
			}
		}

		if ackBehavior == "immediate" {
			sendResponse("ACK\r")
			e.EnqueueMessage(routeID, parsedData)
		} else {
			success, resp, _ := e.ExecuteRoute(routeID, parsedData)
			if success {
				sendResponse(resp)
			} else {
				sendResponse("ERR\r")
			}
		}
	}
}

func parseMLLP(input string) (bool, string) {
	isMllp := false
	if strings.HasPrefix(input, "\x0B") {
		isMllp = true
		input = input[1:]
	}
	if idx := strings.Index(input, "\x1C"); idx != -1 {
		input = input[:idx]
	}
	return isMllp, input
}



func (e *IntegrationEngine) ExecuteRoute(routeID string, inputData string) (bool, string, error) {
	route, exists := e.Routes[routeID]
	if !exists {
		return false, "", fmt.Errorf("Route not found")
	}

	log.Printf("[Route %s] Received inbound data", route.Name)
	transformedData := inputData
	status := "SUCCESS"
	errorMsg := ""
	targetResponse := ""

	source := e.GetEndpoint(route, "source")
	targets := e.GetEndpointsByRole(route, "target")
	sourceProto, targetProto := determineProtocols(source, targets)

	defer func() {
		id := uuid.New().String()
		_, err := db.Exec(`INSERT INTO message_logs (id, route_id, source_protocol, target_protocol, inbound_data, outbound_data, response_data, status, error_message, created_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, routeID, sourceProto, targetProto, inputData, transformedData, targetResponse, status, errorMsg, time.Now().Format(time.RFC3339))
		if err != nil {
			log.Println("DB Log error:", err)
		}
	}()

	// 1. Source Filter Check
	passed, err := checkSourceFilter(source, inputData)
	if err != nil {
		status = "ERROR"
		errorMsg = "Source Filter Error: " + err.Error()
		return false, "", err
	}
	if !passed {
		status = "FILTERED"
		targetResponse = "Filtered by Source Filter"
		log.Printf("[Route %s] Message filtered out by Source Filter", route.Name)
		return true, targetResponse, nil
	}

	// 2. Source Inbound Transformer
	transformedData, err = runSourceTransformer(source, route, inputData)
	if err != nil {
		status = "ERROR"
		errorMsg = "Source Transformer Error: " + err.Error()
		return false, "", err
	}

	if len(targets) == 0 {
		status = "ERROR"
		errorMsg = "No target destinations configured"
		return false, "", errors.New(errorMsg)
	}

	// 3. Multi-Destination Dispatch Loop
	responses, errorsList, dispatchedCount := e.dispatchAllTargets(targets, transformedData, route)

	if len(errorsList) > 0 {
		if dispatchedCount == 0 {
			status = "ERROR"
		} else {
			status = "PARTIAL_SUCCESS"
		}
		errorMsg = strings.Join(errorsList, " | ")
	}

	targetResponse = strings.Join(responses, "\n")
	if dispatchedCount == 0 && status != "ERROR" && status != "FILTERED" {
		targetResponse = "All destinations filtered or disabled"
	}

	return status == "SUCCESS" || status == "PARTIAL_SUCCESS", targetResponse, nil
}

func determineProtocols(source *Endpoint, targets []Endpoint) (string, string) {
	sourceProto := "unknown"
	if source != nil {
		sourceProto = source.Protocol
	}
	targetProto := "multi"
	if len(targets) == 1 {
		targetProto = targets[0].Protocol
	} else if len(targets) == 0 {
		targetProto = "none"
	}
	return sourceProto, targetProto
}

func checkSourceFilter(source *Endpoint, inputData string) (bool, error) {
	if source == nil {
		return true, nil
	}
	var srcCfg map[string]interface{}
	json.Unmarshal([]byte(source.Config), &srcCfg)
	filterScript, ok := srcCfg["filter_script"].(string)
	if !ok || strings.TrimSpace(filterScript) == "" {
		return true, nil
	}
	return evalFilterScript(filterScript, inputData)
}

func runSourceTransformer(source *Endpoint, route Route, inputData string) (string, error) {
	sourceScript := route.Transformer.SourceScript
	if source != nil {
		var srcCfg map[string]interface{}
		json.Unmarshal([]byte(source.Config), &srcCfg)
		if s, ok := srcCfg["transformer_script"].(string); ok && strings.TrimSpace(s) != "" {
			sourceScript = s
		}
	}
	if strings.TrimSpace(sourceScript) == "" {
		return inputData, nil
	}
	return runTransformerScript(sourceScript, inputData, route.Transformer.InboundTemplate, route.Transformer.OutboundTemplate)
}

func (e *IntegrationEngine) dispatchAllTargets(targets []Endpoint, transformedData string, route Route) ([]string, []string, int) {
	var responses []string
	var errorsList []string
	dispatchedCount := 0

	for idx, tgt := range targets {
		destName, resp, err, skipped := e.processDestination(tgt, idx, transformedData, route)
		if skipped {
			continue
		}
		if err != nil {
			errorsList = append(errorsList, err.Error())
		} else {
			dispatchedCount++
			responses = append(responses, fmt.Sprintf("[%s]: %s", destName, resp))
		}
	}
	return responses, errorsList, dispatchedCount
}

func (e *IntegrationEngine) processDestination(tgt Endpoint, idx int, transformedData string, route Route) (string, string, error, bool) {
	var targetCfg map[string]interface{}
	json.Unmarshal([]byte(tgt.Config), &targetCfg)

	destName, _ := targetCfg["name"].(string)
	if destName == "" {
		destName = fmt.Sprintf("Destination %d", idx+1)
	}

	if enabled, ok := targetCfg["enabled"].(bool); ok && !enabled {
		log.Printf("[Route %s] Destination '%s' is disabled, skipping", route.Name, destName)
		return destName, "", nil, true
	}

	if filterScript, ok := targetCfg["filter_script"].(string); ok && strings.TrimSpace(filterScript) != "" {
		passed, err := evalFilterScript(filterScript, transformedData)
		if err != nil {
			return destName, "", fmt.Errorf("[%s] Filter error: %w", destName, err), false
		}
		if !passed {
			log.Printf("[Route %s] Destination '%s' filtered out message", route.Name, destName)
			return destName, "", nil, true
		}
	}

	destPayload, err := applyDestinationTransformer(targetCfg, transformedData, route)
	if err != nil {
		return destName, "", fmt.Errorf("[%s] Transformer error: %w", destName, err), false
	}

	resp, err := e.dispatchTargetProtocol(tgt, targetCfg, destPayload, route)
	if err != nil {
		return destName, "", fmt.Errorf("[%s] %w", destName, err), false
	}
	return destName, resp, nil, false
}

func applyDestinationTransformer(targetCfg map[string]interface{}, transformedData string, route Route) (string, error) {
	destScript := route.Transformer.Script
	if s, ok := targetCfg["transformer_script"].(string); ok && strings.TrimSpace(s) != "" {
		destScript = s
	}
	destOutTemplate := route.Transformer.OutboundTemplate
	if t, ok := targetCfg["outbound_template"].(string); ok && strings.TrimSpace(t) != "" {
		destOutTemplate = t
	}

	if strings.TrimSpace(destScript) == "" {
		return transformedData, nil
	}
	return runTransformerScript(destScript, transformedData, route.Transformer.InboundTemplate, destOutTemplate)
}

func (e *IntegrationEngine) dispatchTargetProtocol(tgt Endpoint, targetCfg map[string]interface{}, destPayload string, route Route) (string, error) {
	switch tgt.Protocol {
	case "http":
		return e.dispatchHTTP(targetCfg, destPayload, route)
	case "internal-channel":
		targetRouteID, _ := targetCfg["route_id"].(string)
		if targetRouteID == "" {
			return "", errors.New("Target Route ID is missing")
		}
		err := e.EnqueueMessage(targetRouteID, destPayload)
		if err != nil {
			return "", err
		}
		return "Enqueued to Route ID: " + targetRouteID, nil
	case "tcp":
		return e.dispatchTCP(targetCfg, &tgt, destPayload, route)
	default:
		return "", fmt.Errorf("Unsupported protocol: %s", tgt.Protocol)
	}
}

func (e *IntegrationEngine) dispatchHTTP(targetCfg map[string]interface{}, payload string, route Route) (string, error) {
	url := ""
	if u, ok := targetCfg["url"].(string); ok && u != "" {
		url = u
	} else {
		host, _ := targetCfg["host"].(string)
		if host == "" { host = "localhost" }
		portStr := ""
		if p, ok := targetCfg["port"].(string); ok && p != "" { portStr = ":" + p }
		path, _ := targetCfg["path"].(string)
		url = "http://" + host + portStr + path
	}
	
	url = e.interpolateVariables(url, route)
	payload = e.interpolateVariables(payload, route)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	
	if authType, ok := targetCfg["authType"].(string); ok {
		switch authType {
		case "basic":
			user, _ := targetCfg["username"].(string)
			pass, _ := targetCfg["password"].(string)
			req.SetBasicAuth(e.interpolateVariables(user, route), e.interpolateVariables(pass, route))
		case "bearer":
			token, _ := targetCfg["token"].(string)
			req.Header.Set("Authorization", "Bearer "+e.interpolateVariables(token, route))
		}
	}
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	return string(bodyBytes), nil
}

func (e *IntegrationEngine) dispatchTCP(targetCfg map[string]interface{}, target *Endpoint, payload string, route Route) (string, error) {
	host, _ := targetCfg["host"].(string)
	port, _ := targetCfg["port"].(string)
	
	host = e.interpolateVariables(host, route)
	port = e.interpolateVariables(port, route)
	payload = e.interpolateVariables(payload, route)
	
	conn, err := net.DialTimeout("tcp", host+":"+port, 5*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	if target.Format == "hl7" || strings.HasPrefix(payload, "MSH") {
		payload = fmt.Sprintf("\x0B%s\x1C\x0D", payload)
	}
	
	conn.Write([]byte(payload))
	
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil {
		return "", err
	}
	
	resText := string(buf[:n])
	resText = strings.TrimPrefix(resText, "\x0B")
	if idx := strings.Index(resText, "\x1C"); idx != -1 { resText = resText[:idx] }
	
	return resText, nil
}
