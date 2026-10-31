package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ConnectorType struct {
	ID           string `json:"id"`
	Category     string `json:"category"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SampleConfig string `json:"sample_config"`
	Active       bool   `json:"active"`
}

type Connector struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Role        string                 `json:"role"`
	Protocol    string                 `json:"protocol"`
	Format      string                 `json:"format"`
	Config      map[string]interface{} `json:"config"`
	Description string                 `json:"description"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// ----------------------------------------------------------------------------
// CONNECTOR REGISTRY HANDLERS (Dynamic Protocol & Driver Metadata)
// ----------------------------------------------------------------------------

func getConnectorRegistry(c *gin.Context) {
	rows, err := db.Query("SELECT id, category, code, name, description, sample_config, active FROM connector_types WHERE active = 1 ORDER BY category, name")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	protocols := make([]ConnectorType, 0)
	drivers := make([]ConnectorType, 0)
	formats := make([]ConnectorType, 0)

	for rows.Next() {
		var ct ConnectorType
		var desc, sample sql.NullString
		var active sql.NullBool
		if err := rows.Scan(&ct.ID, &ct.Category, &ct.Code, &ct.Name, &desc, &sample, &active); err == nil {
			ct.Description = desc.String
			ct.SampleConfig = sample.String
			ct.Active = active.Valid && active.Bool

			switch ct.Category {
			case "protocol":
				protocols = append(protocols, ct)
			case "database_driver":
				drivers = append(drivers, ct)
			case "format":
				formats = append(formats, ct)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"protocols":        protocols,
		"database_drivers": drivers,
		"formats":          formats,
	})
}

func createConnectorType(c *gin.Context) {
	var input struct {
		Category     string `json:"category" binding:"required"`
		Code         string `json:"code" binding:"required"`
		Name         string `json:"name" binding:"required"`
		Description  string `json:"description"`
		SampleConfig string `json:"sample_config"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload: " + err.Error()})
		return
	}

	code := strings.ToLower(strings.TrimSpace(input.Code))
	id := fmt.Sprintf("%s-%s", input.Category, code)

	_, err := db.Exec("INSERT INTO connector_types (id, category, code, name, description, sample_config, active) VALUES (?, ?, ?, ?, ?, ?, 1)",
		id, input.Category, code, input.Name, input.Description, input.SampleConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create connector type: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id, "message": "Connector type created successfully"})
}

func updateConnectorType(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		SampleConfig string `json:"sample_config"`
		Active       *bool  `json:"active"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload: " + err.Error()})
		return
	}

	activeVal := 1
	if input.Active != nil && !*input.Active {
		activeVal = 0
	}

	_, err := db.Exec("UPDATE connector_types SET name = ?, description = ?, sample_config = ?, active = ? WHERE id = ?",
		input.Name, input.Description, input.SampleConfig, activeVal, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update connector type: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Connector type updated successfully"})
}

func deleteConnectorType(c *gin.Context) {
	id := c.Param("id")
	_, err := db.Exec("DELETE FROM connector_types WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete connector type: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Connector type deleted"})
}

// ----------------------------------------------------------------------------
// REUSABLE CONNECTORS (Connection Profiles)
// ----------------------------------------------------------------------------

func getConnectors(c *gin.Context) {
	roleFilter := c.Query("role")

	query := "SELECT id, name, role, protocol, format, config, description, created_at, updated_at FROM connectors"
	var args []interface{}
	if roleFilter != "" {
		query += " WHERE role = ? OR role = 'shared'"
		args = append(args, roleFilter)
	}
	query += " ORDER BY name ASC"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	connectors := make([]Connector, 0)
	for rows.Next() {
		var conn Connector
		var configStr, desc, createdAt, updatedAt sql.NullString
		if err := rows.Scan(&conn.ID, &conn.Name, &conn.Role, &conn.Protocol, &conn.Format, &configStr, &desc, &createdAt, &updatedAt); err == nil {
			conn.Description = desc.String
			conn.CreatedAt = createdAt.String
			conn.UpdatedAt = updatedAt.String

			if configStr.Valid && configStr.String != "" {
				_ = json.Unmarshal([]byte(configStr.String), &conn.Config)
			}
			if conn.Config == nil {
				conn.Config = make(map[string]interface{})
			}
			connectors = append(connectors, conn)
		}
	}

	c.JSON(http.StatusOK, connectors)
}

func getConnector(c *gin.Context) {
	id := c.Param("id")
	var conn Connector
	var configStr, desc, createdAt, updatedAt sql.NullString
	err := db.QueryRow("SELECT id, name, role, protocol, format, config, description, created_at, updated_at FROM connectors WHERE id = ?", id).
		Scan(&conn.ID, &conn.Name, &conn.Role, &conn.Protocol, &conn.Format, &configStr, &desc, &createdAt, &updatedAt)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connector not found"})
		return
	}

	conn.Description = desc.String
	conn.CreatedAt = createdAt.String
	conn.UpdatedAt = updatedAt.String
	if configStr.Valid && configStr.String != "" {
		_ = json.Unmarshal([]byte(configStr.String), &conn.Config)
	}
	if conn.Config == nil {
		conn.Config = make(map[string]interface{})
	}

	c.JSON(http.StatusOK, conn)
}

func createConnector(c *gin.Context) {
	var input struct {
		Name        string                 `json:"name" binding:"required"`
		Role        string                 `json:"role"`
		Protocol    string                 `json:"protocol" binding:"required"`
		Format      string                 `json:"format"`
		Config      map[string]interface{} `json:"config"`
		Description string                 `json:"description"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload: " + err.Error()})
		return
	}

	if input.Role == "" {
		input.Role = "shared"
	}
	if input.Format == "" {
		input.Format = "json"
	}
	if input.Config == nil {
		input.Config = make(map[string]interface{})
	}

	configJSON, _ := json.Marshal(input.Config)
	id := uuid.New().String()
	now := time.Now().Format("2006-01-02 15:04:05")

	_, err := db.Exec("INSERT INTO connectors (id, name, role, protocol, format, config, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, input.Name, input.Role, input.Protocol, input.Format, string(configJSON), input.Description, now, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create connector: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id, "message": "Connector saved successfully"})
}

func updateConnector(c *gin.Context) {
	id := c.Param("id")
	var input struct {
		Name        string                 `json:"name" binding:"required"`
		Role        string                 `json:"role"`
		Protocol    string                 `json:"protocol" binding:"required"`
		Format      string                 `json:"format"`
		Config      map[string]interface{} `json:"config"`
		Description string                 `json:"description"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload: " + err.Error()})
		return
	}

	if input.Role == "" {
		input.Role = "shared"
	}
	if input.Format == "" {
		input.Format = "json"
	}
	if input.Config == nil {
		input.Config = make(map[string]interface{})
	}

	configJSON, _ := json.Marshal(input.Config)
	now := time.Now().Format("2006-01-02 15:04:05")

	_, err := db.Exec("UPDATE connectors SET name = ?, role = ?, protocol = ?, format = ?, config = ?, description = ?, updated_at = ? WHERE id = ?",
		input.Name, input.Role, input.Protocol, input.Format, string(configJSON), input.Description, now, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update connector: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Connector updated successfully"})
}

func deleteConnector(c *gin.Context) {
	id := c.Param("id")
	_, err := db.Exec("DELETE FROM connectors WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete connector: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Connector deleted successfully"})
}

func testConnectorProfile(c *gin.Context) {
	id := c.Param("id")
	var conn Connector
	var configStr sql.NullString
	err := db.QueryRow("SELECT id, name, role, protocol, config FROM connectors WHERE id = ?", id).
		Scan(&conn.ID, &conn.Name, &conn.Role, &conn.Protocol, &configStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Connector not found"})
		return
	}

	cfg := make(map[string]interface{})
	if configStr.Valid && configStr.String != "" {
		_ = json.Unmarshal([]byte(configStr.String), &cfg)
	}

	if conn.Role == "target" {
		testTargetConnection(c, conn.Protocol, cfg, "")
	} else {
		testSourceConnection(c, conn.Protocol, cfg, "")
	}
}
