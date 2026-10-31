package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	api := r.Group("/api")
	{
		api.GET("/connector-registry", getConnectorRegistry)
		api.POST("/connector-registry", createConnectorType)
		api.PUT("/connector-registry/:id", updateConnectorType)
		api.DELETE("/connector-registry/:id", deleteConnectorType)

		api.GET("/connectors", getConnectors)
		api.POST("/connectors", createConnector)
		api.GET("/connectors/:id", getConnector)
		api.PUT("/connectors/:id", updateConnector)
		api.DELETE("/connectors/:id", deleteConnector)
	}
	return r
}

func TestConnectorRegistryAndProfiles(t *testing.T) {
	if err := initDb(); err != nil {
		t.Fatalf("initDb failed: %v", err)
	}

	router := setupTestRouter()

	// 1. Test GET /api/connector-registry
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/connector-registry", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var regData struct {
		Protocols       []ConnectorType `json:"protocols"`
		DatabaseDrivers []ConnectorType `json:"database_drivers"`
		Formats         []ConnectorType `json:"formats"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &regData); err != nil {
		t.Fatalf("Failed to parse registry JSON: %v", err)
	}

	if len(regData.Protocols) == 0 {
		t.Errorf("Expected protocols in registry, got 0")
	}
	if len(regData.DatabaseDrivers) == 0 {
		t.Errorf("Expected database drivers in registry, got 0")
	}

	t.Logf("Registry loaded: %d protocols, %d drivers, %d formats",
		len(regData.Protocols), len(regData.DatabaseDrivers), len(regData.Formats))

	// 2. Test POST /api/connectors (Create new Reusable Connector)
	newConn := map[string]interface{}{
		"name":        "Test SIMRS Reusable Connector",
		"role":        "source",
		"protocol":    "database",
		"format":      "json",
		"description": "Konektor tes unit untuk SIMRS",
		"config": map[string]interface{}{
			"driver":       "odbc",
			"dsn":          "DSN=TEST_SIMRS;UID=test;PWD=secret;",
			"pollInterval": "30",
		},
	}
	payloadBytes, _ := json.Marshal(newConn)

	wPost := httptest.NewRecorder()
	reqPost, _ := http.NewRequest("POST", "/api/connectors", bytes.NewBuffer(payloadBytes))
	reqPost.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(wPost, reqPost)

	if wPost.Code != http.StatusOK {
		t.Fatalf("Failed to create connector: %s", wPost.Body.String())
	}

	var postRes map[string]string
	json.Unmarshal(wPost.Body.Bytes(), &postRes)
	createdId := postRes["id"]
	if createdId == "" {
		t.Fatalf("Expected created connector ID, got empty")
	}
	t.Logf("Created reusable connector: ID=%s", createdId)

	// 3. Test GET /api/connectors/:id
	wGet := httptest.NewRecorder()
	reqGet, _ := http.NewRequest("GET", "/api/connectors/"+createdId, nil)
	router.ServeHTTP(wGet, reqGet)

	if wGet.Code != http.StatusOK {
		t.Fatalf("Failed to get connector: %s", wGet.Body.String())
	}

	var conn Connector
	json.Unmarshal(wGet.Body.Bytes(), &conn)
	if conn.Name != "Test SIMRS Reusable Connector" {
		t.Errorf("Expected connector name 'Test SIMRS Reusable Connector', got '%s'", conn.Name)
	}

	// 4. Test DELETE /api/connectors/:id
	wDel := httptest.NewRecorder()
	reqDel, _ := http.NewRequest("DELETE", "/api/connectors/"+createdId, nil)
	router.ServeHTTP(wDel, reqDel)

	if wDel.Code != http.StatusOK {
		t.Fatalf("Failed to delete connector: %s", wDel.Body.String())
	}
	t.Log("Reusable connector test passed successfully!")
}
