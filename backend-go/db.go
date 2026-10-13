package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/denisenkom/go-mssqldb"
	_ "modernc.org/sqlite"
	_ "github.com/SAP/go-hdb/driver"
	_ "github.com/alexbrainman/odbc"
)

var db *sql.DB
var dbType string

const queryInsertGlobalVar = "INSERT INTO global_variables (id, key_name, value_data, is_secret, description) VALUES (?, ?, ?, ?, ?)"

func initDb() error {
	dbType = os.Getenv("DB_TYPE")
	if dbType == "" {
		dbType = "sqlite"
	}

	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")

	var dsn string
	var driver string

	switch dbType {
	case "mysql":
		driver = "mysql"
		if dbName == "" { dbName = "middleware" }
		dsn = fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true", dbUser, dbPass, dbHost, dbName)
	case "postgres":
		driver = "postgres"
		if dbName == "" { dbName = "middleware" }
		dsn = fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", dbUser, dbPass, dbHost, dbName)
	default:
		driver = "sqlite"
		if dbName == "" { dbName = "middleware.db" } // Portable default
		dsn = dbName
	}

	var err error
	db, err = sql.Open(driver, dsn)
	if err != nil {
		return err
	}

	varchar := "VARCHAR(255)"
	if dbType == "postgres" {
		varchar = "VARCHAR"
	}

	queries := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS global_variables (
			id %s PRIMARY KEY,
			key_name %s UNIQUE,
			value_data TEXT,
			is_secret INTEGER DEFAULT 0,
			description TEXT
		)`, varchar, varchar),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS route_groups (
			id %s PRIMARY KEY,
			name %s UNIQUE,
			description TEXT
		)`, varchar, varchar),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS routes (
			id %s PRIMARY KEY,
			name %s,
			description TEXT,
			group_name TEXT,
			active INTEGER DEFAULT 1,
			variables TEXT DEFAULT '{}'
		)`, varchar, varchar),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS endpoints (
			id %s PRIMARY KEY,
			route_id %s,
			role %s,
			protocol %s,
			format %s,
			config TEXT
		)`, varchar, varchar, varchar, varchar, varchar),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS transformers (
			id %s PRIMARY KEY,
			route_id %s,
			script TEXT,
			inbound_template TEXT,
			outbound_template TEXT
		)`, varchar, varchar),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS message_logs (
			id %s PRIMARY KEY,
			route_id %s,
			source_protocol %s,
			target_protocol %s,
			inbound_data TEXT,
			outbound_data TEXT,
			response_data TEXT,
			status %s,
			error_message TEXT,
			created_at %s
		)`, varchar, varchar, varchar, varchar, varchar, varchar),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS users (
			id %s PRIMARY KEY,
			username %s UNIQUE,
			password_hash %s,
			active BOOLEAN DEFAULT 1
		)`, varchar, varchar, varchar),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS queue_messages (
			id %s PRIMARY KEY,
			route_id %s,
			payload TEXT,
			status %s,
			retry_count INTEGER DEFAULT 0,
			next_retry_at %s,
			created_at %s
		)`, varchar, varchar, varchar, varchar, varchar),
	}

	for _, q := range queries {
		_, err := db.Exec(q)
		if err != nil {
			log.Printf("DB Init Error: %v", err)
		}
	}
	
	// Ensure missing columns exist if reading from old sqlite DB
	db.Exec("ALTER TABLE message_logs ADD COLUMN response_data TEXT")
	db.Exec("ALTER TABLE routes ADD COLUMN group_name TEXT")
	db.Exec("ALTER TABLE routes ADD COLUMN variables TEXT DEFAULT '{}'")
	db.Exec("ALTER TABLE transformers ADD COLUMN source_script TEXT DEFAULT ''")

	// High-performance indexes: speed up stats aggregation and log filtering without full table scans
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_message_logs_route_status ON message_logs(route_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_message_logs_created_at ON message_logs(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_queue_messages_status_route ON queue_messages(status, route_id)",
	}
	for _, idx := range indexes {
		if _, err := db.Exec(idx); err != nil {
			log.Printf("Index initialization note: %v", err)
		}
	}

	// Ensure default admin user exists
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count == 0 {
		hashed, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		db.Exec("INSERT INTO users (id, username, password_hash, active) VALUES (?, ?, ?, ?)",
			uuid.New().String(), "admin", string(hashed), true)
	}

	seedDefaultSatusehatTemplate()

	return nil
}

func seedDefaultSatusehatTemplate() {
	// 1. Group
	var grpCount int
	db.QueryRow("SELECT COUNT(*) FROM route_groups WHERE name = 'SATUSEHAT KEMENKES'").Scan(&grpCount)
	if grpCount == 0 {
		db.Exec("INSERT INTO route_groups (id, name, description) VALUES (?, ?, ?)",
			uuid.New().String(), "SATUSEHAT KEMENKES", "Integrasi SATUSEHAT Kemenkes FHIR R4 Multi-Tenant")
	}

	// 2. Global Variable: TENANT_CREDENTIALS
	var varCount int
	db.QueryRow("SELECT COUNT(*) FROM global_variables WHERE key_name = 'TENANT_CREDENTIALS'").Scan(&varCount)
	if varCount == 0 {
		sampleJSON := `{
  "RS01": {
    "name": "RS Contoh 1",
    "satusehat": {
      "org_id": "10000001",
      "client_id": "YOUR_STAGING_CLIENT_ID_RS01",
      "client_secret": "YOUR_STAGING_CLIENT_SECRET_RS01"
    }
  },
  "RS02": {
    "name": "RS Contoh 2",
    "satusehat": {
      "org_id": "10000002",
      "client_id": "YOUR_STAGING_CLIENT_ID_RS02",
      "client_secret": "YOUR_STAGING_CLIENT_SECRET_RS02"
    }
  }
}`
		encryptedVal, err := EncryptSecret(sampleJSON)
		if err != nil {
			encryptedVal = sampleJSON
		}
		db.Exec(queryInsertGlobalVar,
			uuid.New().String(), "TENANT_CREDENTIALS", encryptedVal, 1, "Kredensial Multi-Tenant untuk SATUSEHAT / Asuransi (Terkunci AES-256-GCM)")
	}

	// 2b. Global Variable: SATUSEHAT_AUTH_URL
	var authUrlCount int
	db.QueryRow("SELECT COUNT(*) FROM global_variables WHERE key_name = 'SATUSEHAT_AUTH_URL'").Scan(&authUrlCount)
	if authUrlCount == 0 {
		db.Exec(queryInsertGlobalVar,
			uuid.New().String(), "SATUSEHAT_AUTH_URL", "https://api-satusehat-stg.kemkes.go.id/oauth2/v1/accesstoken?grant_type=client_credentials", 0, "URL OAuth2 Access Token SATUSEHAT Kemenkes (Staging / Production)")
	}

	// 2c. Global Variable: SATUSEHAT_FHIR_URL
	var fhirUrlCount int
	db.QueryRow("SELECT COUNT(*) FROM global_variables WHERE key_name = 'SATUSEHAT_FHIR_URL'").Scan(&fhirUrlCount)
	if fhirUrlCount == 0 {
		db.Exec(queryInsertGlobalVar,
			uuid.New().String(), "SATUSEHAT_FHIR_URL", "https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1", 0, "Base URL FHIR R4 Resources SATUSEHAT (Staging / Production)")
	}

	// 3. Default Gateway Route
	defaultScript := `// ============================================================================
// SATUSEHAT MULTI-TENANT DYNAMIC TRANSFORMER
// ============================================================================

// 1. Parse Inbound JSON Payload from SIMRS
var raw = JSON.parse(input);

var hospitalCode = raw.hospital_code || raw.hospitalCode || raw.tenant_id;
var resourceType = raw.resource || raw.resourceType;
var httpMethod = (raw.method || "POST").toUpperCase();
var resourceId = raw.resource_id || raw.id || "";
var fhirData = raw.data || raw.payload || raw;

if (!hospitalCode) {
    throw new Error("Missing hospital_code in request payload");
}
if (!resourceType) {
    throw new Error("Missing resource type in request payload (e.g. Encounter, CarePlan)");
}

// 2. Lookup Hospital Credentials from Encrypted Global Variable
var tenantsJson = getGlobalVar("TENANT_CREDENTIALS");
if (!tenantsJson) {
    throw new Error("TENANT_CREDENTIALS variable not found in Global Variables");
}

var tenants = JSON.parse(tenantsJson);
var tenant = tenants[hospitalCode];
if (!tenant) {
    throw new Error("Hospital code '" + hospitalCode + "' is not registered in TENANT_CREDENTIALS");
}

var satusehatCfg = tenant.satusehat || tenant;
var clientId = satusehatCfg.client_id;
var clientSecret = satusehatCfg.client_secret;
var orgId = satusehatCfg.org_id;

if (!clientId || !clientSecret) {
    throw new Error("Client ID or Client Secret missing for hospital: " + hospitalCode);
}

// 3. Obtain OAuth2 Access Token (In-Memory Cache with TTL)
var cacheKey = "satusehat_token_" + hospitalCode;
var token = cache.get(cacheKey);

if (!token) {
    var authUrl = getGlobalVar("SATUSEHAT_AUTH_URL") || "https://api-satusehat-stg.kemkes.go.id/oauth2/v1/accesstoken?grant_type=client_credentials";
    var formBody = "client_id=" + encodeURIComponent(clientId) + "&client_secret=" + encodeURIComponent(clientSecret);
    
    var authRes = http.request({
        url: authUrl,
        method: "POST",
        headers: {
            "Content-Type": "application/x-www-form-urlencoded"
        },
        body: formBody,
        timeout: 15
    });

    if (authRes.status !== 200) {
        throw new Error("Failed to authenticate with SATUSEHAT for " + hospitalCode + ": " + authRes.status + " " + authRes.body);
    }

    var authData = JSON.parse(authRes.body);
    if (!authData.access_token) {
        throw new Error("SATUSEHAT Auth response did not contain access_token: " + authRes.body);
    }

    token = authData.access_token;
    var expiresIn = parseInt(authData.expires_in, 10) || 3000;
    var cacheTtl = Math.max(300, expiresIn - 300);
    cache.set(cacheKey, token, cacheTtl);
}

// 4. Enrich Organization ID if needed
if (orgId && typeof fhirData === 'object' && fhirData !== null) {
    if (resourceType === 'Encounter' && !fhirData.serviceProvider) {
        fhirData.serviceProvider = {
            "reference": "Organization/" + orgId
        };
    }
}

// 5. Construct Dynamic Target URL from Config Variable
var rawBaseUrl = getGlobalVar("SATUSEHAT_FHIR_URL") || "https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1";
var baseUrl = rawBaseUrl.replace(/\/+$/, "");
var targetUrl = baseUrl + "/" + resourceType;
if (resourceId && (httpMethod === "PUT" || httpMethod === "GET")) {
    targetUrl += "/" + resourceId;
}

// 6. Return Dynamic Dispatch Envelope to Engine
return JSON.stringify({
    __dispatch: {
        url: targetUrl,
        method: httpMethod,
        headers: {
            "Authorization": "Bearer " + token,
            "Content-Type": "application/json"
        }
    },
    payload: fhirData
});`

	var routeCount int
	db.QueryRow("SELECT COUNT(*) FROM routes WHERE id = 'satusehat' OR name = 'SATUSEHAT Multi-Tenant Gateway'").Scan(&routeCount)
	if routeCount == 0 {
		routeID := "satusehat"
		db.Exec("INSERT INTO routes (id, name, description, group_name, active, variables) VALUES (?, ?, ?, ?, ?, ?)",
			routeID, "SATUSEHAT Multi-Tenant Gateway", "1 Ingress Gateway untuk 10 RS & 19 Resources SATUSEHAT Staging", "SATUSEHAT KEMENKES", 1, "{}")

		// Source Endpoint
		db.Exec("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, ?, ?, ?, ?)",
			uuid.New().String(), routeID, "source", "http", "json", `{"ackBehavior":"deferred"}`)

		// Target Endpoint
		db.Exec("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, ?, ?, ?, ?)",
			uuid.New().String(), routeID, "target", "http", "json", `{"name":"SATUSEHAT Gateway","enabled":true,"url":"https://api-satusehat-stg.kemkes.go.id"}`)

		db.Exec("INSERT INTO transformers (id, route_id, source_script, script, inbound_template, outbound_template) VALUES (?, ?, ?, ?, ?, ?)",
			uuid.New().String(), routeID, defaultScript, defaultScript, "", "")
	} else {
		// Update transformer script to support dynamic SATUSEHAT_AUTH_URL and SATUSEHAT_FHIR_URL
		db.Exec("UPDATE transformers SET source_script = ?, script = ? WHERE route_id = 'satusehat'", defaultScript, defaultScript)
	}
}
