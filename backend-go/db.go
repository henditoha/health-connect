package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

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
	loadDotEnv()
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
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS connector_types (
			id %s PRIMARY KEY,
			category %s,
			code %s,
			name %s,
			description TEXT,
			sample_config TEXT,
			active BOOLEAN DEFAULT 1,
			UNIQUE(category, code)
		)`, varchar, varchar, varchar, varchar),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS connectors (
			id %s PRIMARY KEY,
			name %s,
			role %s,
			protocol %s,
			format %s,
			config TEXT,
			description TEXT,
			created_at %s,
			updated_at %s
		)`, varchar, varchar, varchar, varchar, varchar, varchar, varchar),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS dicom_worklist_items (
			id %s PRIMARY KEY,
			accession_number %s UNIQUE,
			patient_id %s,
			patient_name %s,
			birth_date %s,
			gender %s,
			modality %s,
			procedure_desc TEXT,
			study_desc TEXT,
			req_proc_desc TEXT,
			series_desc TEXT,
			study_uid %s,
			station_ae_title %s DEFAULT '*',
			physician_name %s,
			scheduled_date %s,
			scheduled_time %s,
			status %s DEFAULT 'SCHEDULED',
			raw_payload TEXT,
			created_at %s,
			updated_at %s
		)`, varchar, varchar, varchar, varchar, varchar, varchar, varchar, varchar, varchar, varchar, varchar, varchar, varchar, varchar, varchar),
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
	db.Exec("ALTER TABLE dicom_worklist_items ADD COLUMN series_desc TEXT DEFAULT ''")

	// Migration: upgrade connector_types to UNIQUE(category, code) if using old single-column unique
	if dbType == "sqlite" {
		var hasOldUnique int
		_ = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='connector_types' AND sql LIKE '%code VARCHAR(255) UNIQUE%'").Scan(&hasOldUnique)
		if hasOldUnique > 0 {
			_, _ = db.Exec(`
				CREATE TABLE connector_types_new (
					id VARCHAR(255) PRIMARY KEY,
					category VARCHAR(255),
					code VARCHAR(255),
					name VARCHAR(255),
					description TEXT,
					sample_config TEXT,
					active BOOLEAN DEFAULT 1,
					UNIQUE(category, code)
				);
				INSERT OR IGNORE INTO connector_types_new SELECT * FROM connector_types;
				DROP TABLE connector_types;
				ALTER TABLE connector_types_new RENAME TO connector_types;
			`)
		}
	}

	// High-performance indexes: speed up stats aggregation and log filtering without full table scans
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_message_logs_route_status ON message_logs(route_id, status)",
		"CREATE INDEX IF NOT EXISTS idx_message_logs_created_at ON message_logs(created_at)",
		"CREATE INDEX IF NOT EXISTS idx_queue_messages_status_route ON queue_messages(status, route_id)",
		"CREATE INDEX IF NOT EXISTS idx_connectors_role ON connectors(role)",
		"CREATE INDEX IF NOT EXISTS idx_connector_types_category ON connector_types(category)",
		"CREATE INDEX IF NOT EXISTS idx_dicom_wl_search ON dicom_worklist_items(scheduled_date, modality, status)",
		"CREATE INDEX IF NOT EXISTS idx_dicom_wl_patient ON dicom_worklist_items(patient_id, accession_number)",
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
	seedDefaultConnectorRegistry()
	seedDefaultRadiologyGateway()

	// Ensure all existing secrets in database are encrypted at rest with AES-256-GCM
	secretRows, err := db.Query("SELECT id, value_data FROM global_variables WHERE is_secret = 1 AND value_data NOT LIKE 'enc:v1:%'")
	if err == nil {
		defer secretRows.Close()
		for secretRows.Next() {
			var sid, sval string
			if err := secretRows.Scan(&sid, &sval); err == nil && sval != "" {
				if enc, err := EncryptSecret(sval); err == nil {
					db.Exec("UPDATE global_variables SET value_data = ? WHERE id = ?", enc, sid)
				}
			}
		}
	}

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
// SATUSEHAT STG DYNAMIC TRANSFORMER & AUTO REFRESH TOKEN
// Channel: SATUSEHAT Multi-Tenant Gateway
// ============================================================================

// 1. Parse Inbound Request from SIMRS
var raw = JSON.parse(input);

// Extract routing parameters
var hospitalCode = raw.hospital_code || raw.hospitalCode || raw.tenant_id || "";
var resourceType = raw.resource || raw.resourceType || "";
var httpMethod = (raw.method || "POST").toUpperCase();
var resourceId = raw.resource_id || raw.id || "";
var fhirData = raw.data || raw.payload || raw;

if (!resourceType && fhirData && fhirData.resourceType) {
    resourceType = fhirData.resourceType;
}
if (!resourceType) {
    throw new Error("Missing resourceType in request payload (e.g. Encounter, Patient, Condition, Observation)");
}

// 2. Resolve Credentials (org_id, client_id, client_secret)
// Supports both Multi-Tenant (via TENANT_CREDENTIALS) and Single/Default (via Channel Variables)
var orgId = "";
var clientId = "";
var clientSecret = "";

if (hospitalCode) {
    var tenantsJson = getGlobalVar("TENANT_CREDENTIALS");
    if (tenantsJson) {
        try {
            var tenants = JSON.parse(tenantsJson);
            var tenant = tenants[hospitalCode];
            if (tenant) {
                var satusehatCfg = tenant.satusehat || tenant;
                orgId = satusehatCfg.org_id || "";
                clientId = satusehatCfg.client_id || "";
                clientSecret = satusehatCfg.client_secret || "";
            }
        } catch (e) {}
    }
}

if (!clientId || !clientSecret) {
    orgId = orgId || getRouteVar("ORG_ID") || getGlobalVar("SATUSEHAT_ORG_ID") || "";
    clientId = clientId || getRouteVar("CLIENT_ID") || getGlobalVar("SATUSEHAT_CLIENT_ID") || "";
    clientSecret = clientSecret || getRouteVar("CLIENT_SECRET") || getGlobalVar("SATUSEHAT_CLIENT_SECRET") || "";
}

if (!clientId || !clientSecret) {
    throw new Error("SATUSEHAT STG credentials missing: configure ORG_ID, CLIENT_ID, CLIENT_SECRET in Channel Variables or TENANT_CREDENTIALS for: " + (hospitalCode || "DEFAULT"));
}

// 3. Automated OAuth2 Token & Auto-Refresh Management (In-Memory Cache with TTL)
var cacheKey = "satusehat_stg_token_" + (hospitalCode || orgId || clientId);
var token = cache.get(cacheKey);

if (!token) {
    var authUrl = getRouteVar("AUTH_URL") || getGlobalVar("SATUSEHAT_AUTH_URL") || "https://api-satusehat-stg.kemkes.go.id/oauth2/v1/accesstoken?grant_type=client_credentials";
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
        throw new Error("Failed to authenticate with SATUSEHAT STG (" + authRes.status + "): " + authRes.body);
    }

    var authData = JSON.parse(authRes.body);
    if (!authData.access_token) {
        throw new Error("SATUSEHAT STG Auth response missing access_token: " + authRes.body);
    }

    token = authData.access_token;
    var expiresIn = parseInt(authData.expires_in, 10) || 3600;
    // Auto-refresh token 300 seconds (5 mins) before expiry
    var cacheTtl = Math.max(300, expiresIn - 300);
    cache.set(cacheKey, token, cacheTtl);
}

// 4. Enrich FHIR Resource with Organization ID if applicable
if (orgId && typeof fhirData === "object" && fhirData !== null) {
    if (resourceType === "Encounter" && !fhirData.serviceProvider) {
        fhirData.serviceProvider = {
            "reference": "Organization/" + orgId
        };
    }
    // Clean non-standard routing fields so SATUSEHAT receives pure FHIR
    delete fhirData.hospital_code;
    delete fhirData.hospitalCode;
    delete fhirData.tenant_id;
    delete fhirData.resource;
    delete fhirData.resource_id;
    delete fhirData.method;
}

// 5. Build Dynamic SATUSEHAT STG Target URL
var rawBaseUrl = getRouteVar("FHIR_BASE_URL") || getGlobalVar("SATUSEHAT_FHIR_URL") || "https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1";
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

	orgId := os.Getenv("SATUSEHAT_ORG_ID")
	if orgId == "" {
		orgId = "YOUR_SATUSEHAT_ORG_ID"
	}
	clientId := os.Getenv("SATUSEHAT_CLIENT_ID")
	if clientId == "" {
		clientId = "YOUR_SATUSEHAT_CLIENT_ID"
	}
	clientSecret := os.Getenv("SATUSEHAT_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = "YOUR_SATUSEHAT_CLIENT_SECRET"
	}
	defaultVars := fmt.Sprintf(`{"ORG_ID":"%s","CLIENT_ID":"%s","CLIENT_SECRET":"%s","AUTH_URL":"https://api-satusehat-stg.kemkes.go.id/oauth2/v1/accesstoken?grant_type=client_credentials","FHIR_BASE_URL":"https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1","ENV":"STAGING"}`, orgId, clientId, clientSecret)
	targetConfig := `{"name":"SATUSEHAT STG","enabled":true,"url":"https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1","host":"https://api-satusehat-stg.kemkes.go.id","path":"/fhir-r4/v1","authType":"bearer","token":"{{SATUSEHAT_TOKEN}}","org_id":"{{ORG_ID}}","client_id":"{{CLIENT_ID}}","client_secret":"{{CLIENT_SECRET}}"}`

	var routeCount int
	db.QueryRow("SELECT COUNT(*) FROM routes WHERE id = 'satusehat' OR name = 'SATUSEHAT Multi-Tenant Gateway'").Scan(&routeCount)
	if routeCount == 0 {
		routeID := "satusehat"
		db.Exec("INSERT INTO routes (id, name, description, group_name, active, variables) VALUES (?, ?, ?, ?, ?, ?)",
			routeID, "SATUSEHAT Multi-Tenant Gateway", "1 Ingress Gateway untuk Multi-RS & 19 Resources SATUSEHAT Staging dengan Token Auto-Refresh", "SATUSEHAT KEMENKES", 1, defaultVars)

		// Source Endpoint
		db.Exec("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, ?, ?, ?, ?)",
			uuid.New().String(), routeID, "source", "http", "json", `{"path":"/ingress/satusehat","ackBehavior":"deferred"}`)

		// Target Endpoint: SATUSEHAT STG
		db.Exec("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, ?, ?, ?, ?)",
			uuid.New().String(), routeID, "target", "http", "json", targetConfig)

		db.Exec("INSERT INTO transformers (id, route_id, source_script, script, inbound_template, outbound_template) VALUES (?, ?, ?, ?, ?, ?)",
			uuid.New().String(), routeID, defaultScript, defaultScript, "{}", "{}")
	} else {
		// Update existing route configuration
		db.Exec("UPDATE routes SET name = 'SATUSEHAT Multi-Tenant Gateway', group_name = 'SATUSEHAT KEMENKES' WHERE id = 'satusehat'")
		
		// Set default variables if empty
		var currentVars string
		db.QueryRow("SELECT variables FROM routes WHERE id = 'satusehat'").Scan(&currentVars)
		if currentVars == "" || currentVars == "{}" {
			db.Exec("UPDATE routes SET variables = ? WHERE id = 'satusehat'", defaultVars)
		}

		// Update or ensure target endpoint has SATUSEHAT STG configuration
		var tgtCount int
		db.QueryRow("SELECT COUNT(*) FROM endpoints WHERE route_id = 'satusehat' AND role = 'target'").Scan(&tgtCount)
		if tgtCount == 0 {
			db.Exec("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, 'satusehat', 'target', 'http', 'json', ?)",
				uuid.New().String(), targetConfig)
		} else {
			db.Exec("UPDATE endpoints SET config = ?, protocol = 'http', format = 'json' WHERE route_id = 'satusehat' AND role = 'target'", targetConfig)
		}

		// Update transformer scripts with auto-refresh token support
		db.Exec("UPDATE transformers SET source_script = ?, script = ? WHERE route_id = 'satusehat'", defaultScript, defaultScript)
	}
}

func loadDotEnv() {
	paths := []string{".env", "backend-go/.env", "../.env"}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					k := strings.TrimSpace(parts[0])
					v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
					if os.Getenv(k) == "" {
						os.Setenv(k, v)
					}
				}
			}
			break
		}
	}
}

func seedDefaultConnectorRegistry() {
	// 1. Seed Connector Types (Protocols, Drivers, Formats)
	var typeCount int
	db.QueryRow("SELECT COUNT(*) FROM connector_types").Scan(&typeCount)
	if typeCount == 0 {
		initialTypes := []struct {
			id, category, code, name, desc, sample string
		}{
			// Protocols
			{"proto-http", "protocol", "http", "HTTP POST Ingress", "Menerima panggilan HTTP POST dari aplikasi external", `{"path":"/ingress/:id"}`},
			{"proto-http-poll", "protocol", "http-poll", "HTTP GET Poll", "Melakukan polling periodik via HTTP GET ke REST API", `{"url":"https://api.hospital.com/data","pollInterval":"60"}`},
			{"proto-tcp", "protocol", "tcp", "TCP / MLLP Socket", "Membuka socket TCP untuk menerima pesan HL7 v2 MLLP", `{"port":"9000"}`},
			{"proto-dicom", "protocol", "dicom", "DICOM (MWL / PACS)", "Server DICOM DIMSE C-ECHO & C-FIND Worklist untuk Modalitas / PACS", `{"port":"11112","ae_title":"HEALTH_CONNECT"}`},
			{"proto-database", "protocol", "database", "Database Poller", "Melakukan polling periodik query SELECT ke basis data", `{"driver":"mysql","pollInterval":"60"}`},
			{"proto-internal", "protocol", "internal", "Internal Queue Reader", "Menerima pesan dari antrean internal rute lain", `{}`},

			// Database Drivers
			{"driver-mysql", "database_driver", "mysql", "MySQL", "Driver native MySQL 5.7+ / 8.0+", `{"dsn":"user:password@tcp(localhost:3306)/dbname?parseTime=true"}`},
			{"driver-mariadb", "database_driver", "mariadb", "MariaDB", "Driver kompatibel MariaDB 10.x+", `{"dsn":"user:password@tcp(localhost:3306)/dbname?parseTime=true"}`},
			{"driver-postgres", "database_driver", "postgres", "PostgreSQL", "Driver PostgreSQL 12+", `{"dsn":"postgres://user:password@localhost:5432/dbname?sslmode=disable"}`},
			{"driver-sqlserver", "database_driver", "sqlserver", "Microsoft SQL Server", "Driver Microsoft SQL Server 2012+", `{"dsn":"sqlserver://user:password@localhost:1433?database=dbname"}`},
			{"driver-sqlite", "database_driver", "sqlite", "SQLite", "File basis data lokal SQLite", `{"dsn":"simrs.db"}`},
			{"driver-hdb", "database_driver", "hdb", "SAP HANA", "Driver SAP HANA Database", `{"dsn":"hdb://user:password@localhost:39015"}`},
			{"driver-odbc", "database_driver", "odbc", "ODBC (Generic) - IRIS, Caché, Oracle, DB2", "Konektor universal ODBC untuk InterSystems IRIS/Caché, Oracle, DB2, dll.", `{"dsn":"DSN=SIMRS_IRIS;UID=user;PWD=pass;"}`},

			// Formats
			{"fmt-json", "format", "json", "JSON", "Format data terstruktur JavaScript Object Notation", `{}`},
			{"fmt-xml", "format", "xml", "XML", "Format dokumen Extensible Markup Language", `{}`},
			{"fmt-hl7", "format", "hl7", "HL7 v2.x ER7", "Format pesan standar kesehatan HL7 v2 pipe-delimited", `{}`},
			{"fmt-fhir", "format", "fhir", "HL7 FHIR R4", "Standar interoperabilitas kesehatan Fast Healthcare Interoperability Resources", `{}`},
			{"fmt-dicom", "format", "dicom", "DICOM", "Format dataset biner medis DICOM PS3.5 / PS3.10", `{}`},
			{"fmt-raw", "format", "raw", "Raw String / Binary", "Teks polos atau data biner", `{}`},
		}

		for _, it := range initialTypes {
			db.Exec("INSERT INTO connector_types (id, category, code, name, description, sample_config, active) VALUES (?, ?, ?, ?, ?, ?, 1)",
				it.id, it.category, it.code, it.name, it.desc, it.sample)
		}
	}

	// Always ensure DICOM protocol and format exist even on previously initialized databases
	db.Exec("INSERT OR IGNORE INTO connector_types (id, category, code, name, description, sample_config, active) VALUES ('proto-dicom', 'protocol', 'dicom', 'DICOM (MWL / PACS)', 'Server DICOM DIMSE C-ECHO & C-FIND Worklist untuk Modalitas / PACS', '{\"port\":\"11112\",\"ae_title\":\"HEALTH_CONNECT\"}', 1)")
	db.Exec("INSERT OR IGNORE INTO connector_types (id, category, code, name, description, sample_config, active) VALUES ('fmt-dicom', 'format', 'dicom', 'DICOM', 'Format dataset biner medis DICOM PS3.5 / PS3.10', '{}', 1)")

	// 2. Seed Reusable Connectors (Connection Profiles)
	var connCount int
	db.QueryRow("SELECT COUNT(*) FROM connectors").Scan(&connCount)
	if connCount == 0 {
		now := time.Now().Format("2006-01-02 15:04:05")
		initialConnectors := []struct {
			id, name, role, protocol, format, config, desc string
		}{
			{
				id:       uuid.New().String(),
				name:     "DICOM Modality Worklist (MWL) & PACS",
				role:     "source",
				protocol: "dicom",
				format:   "dicom",
				config:   `{"port":"11112","ae_title":"HEALTH_CONNECT"}`,
				desc:     "Listener DICOM DIMSE SCP C-ECHO & C-FIND untuk modalitas radiologi (X-Ray, CT, MRI, USG)",
			},
			{
				id:       uuid.New().String(),
				name:     "SATUSEHAT STG FHIR Gateway",
				role:     "target",
				protocol: "http",
				format:   "json",
				config:   `{"name":"SATUSEHAT STG FHIR Gateway","enabled":true,"url":"https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1","host":"https://api-satusehat-stg.kemkes.go.id","path":"/fhir-r4/v1","authType":"bearer","token":"{{SATUSEHAT_TOKEN}}"}`,
				desc:     "Koneksi target gateway ke server SATUSEHAT Staging Kemenkes",
			},
			{
				id:       uuid.New().String(),
				name:     "SIMRS Database (InterSystems / ODBC)",
				role:     "source",
				protocol: "database",
				format:   "json",
				config:   `{"driver":"odbc","dsn":"DSN=SIMRS_IRIS;UID=user;PWD=pass;","pollInterval":"60","query":"SELECT id, norm, nama, poli FROM antrean_poli WHERE status='ready'"}`,
				desc:     "Koneksi polling SQL ke Database SIMRS berbasis ODBC / InterSystems IRIS",
			},
			{
				id:       uuid.New().String(),
				name:     "LIS Alat Lab MLLP (HL7 v2)",
				role:     "source",
				protocol: "tcp",
				format:   "hl7",
				config:   `{"port":"9000","ackBehavior":"deferred"}`,
				desc:     "Listener TCP socket MLLP untuk menerima hasil lab HL7 v2 dari Sysmex/Mindray",
			},
		}

		for _, c := range initialConnectors {
			db.Exec("INSERT INTO connectors (id, name, role, protocol, format, config, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
				c.id, c.name, c.role, c.protocol, c.format, c.config, c.desc, now, now)
		}
	}

	// Always ensure DICOM connector profile exists in connectors
	var dicomConnCount int
	db.QueryRow("SELECT COUNT(*) FROM connectors WHERE protocol = 'dicom'").Scan(&dicomConnCount)
	if dicomConnCount == 0 {
		now := time.Now().Format("2006-01-02 15:04:05")
		db.Exec("INSERT INTO connectors (id, name, role, protocol, format, config, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			uuid.New().String(), "DICOM Modality Worklist (MWL) & PACS", "source", "dicom", "dicom", `{"port":"11112","ae_title":"HEALTH_CONNECT"}`, "Listener DICOM DIMSE SCP C-ECHO & C-FIND untuk modalitas radiologi (X-Ray, CT, MRI, USG)", now, now)
	}

	// Always ensure fmt-auto and proto-database exist in connector types
	db.Exec("INSERT OR IGNORE INTO connector_types (id, category, code, name, description, sample_config, active) VALUES ('fmt-auto', 'format', 'auto', 'Auto-Detect (JSON / HL7)', 'Otomatis mendeteksi payload JSON dari SIMRS atau HL7 v2 (ORM^O01) dari Mirth/TrakCare', '{}', 1)")
	db.Exec("INSERT OR IGNORE INTO connector_types (id, category, code, name, description, sample_config, active) VALUES ('proto-database', 'protocol', 'database', 'Database / SQL Target', 'Menyimpan payload data langsung ke basis data SQL internal atau eksternal', '{\"driver\":\"sqlite\",\"dsn\":\"middleware.db\",\"query\":\"\"}', 1)")
}

func seedDefaultRadiologyGateway() {
	// 1. Group
	var grpCount int
	db.QueryRow("SELECT COUNT(*) FROM route_groups WHERE name = 'RADIOLOGY & PACS'").Scan(&grpCount)
	if grpCount == 0 {
		db.Exec("INSERT INTO route_groups (id, name, description) VALUES (?, ?, ?)",
			uuid.New().String(), "RADIOLOGY & PACS", "Integrasi Radiologi, SIMRS, Mirth Connect, dan DICOM Modality Worklist (MWL)")
	}

	// 2. Ensure Route exists
	var routeCount int
	db.QueryRow("SELECT COUNT(*) FROM routes WHERE id = 'orm-pacs-gateway' OR name LIKE '%Radiology Order%'").Scan(&routeCount)
	if routeCount == 0 {
		routeID := "orm-pacs-gateway"
		routeName := "Radiology Order & PACS Gateway (HL7 / JSON to SQL)"
		routeDesc := "Menerima order radiologi dari SIMRS (JSON) maupun Mirth Connect / TrakCare (HL7 ORM-001) via HTTP & TCP, otomatis transformasi, dan simpan ke basis data SQL (dicom_worklist_items) dalam format JSON"
		groupName := "RADIOLOGY & PACS"

		_, err := db.Exec("INSERT INTO routes (id, name, description, group_name, active, variables) VALUES (?, ?, ?, ?, 1, '{}')",
			routeID, routeName, routeDesc, groupName)
		if err != nil {
			log.Printf("Failed to insert radiology gateway route: %v", err)
			return
		}

		// Source Endpoint: TCP port 6001 with HTTP path fallback /ingress/radiology-orders
		sourceID := uuid.New().String()
		sourceCfg := `{"port":"6001","path":"/ingress/radiology-orders","ackBehavior":"deferred","filter_script":"","transformer_script":""}`
		db.Exec("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, 'source', 'tcp', 'auto', ?)",
			sourceID, routeID, sourceCfg)

		// Target Endpoint: DICOM Modality Worklist (SQL)
		targetID := uuid.New().String()
		targetCfg := `{"name":"DICOM Modality Worklist (SQL)","enabled":true,"filter_script":"","transformer_script":""}`
		db.Exec("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, 'target', 'dicom', 'dicom', ?)",
			targetID, routeID, targetCfg)

		// Transformer Script: Auto-detects JSON / HL7 v2 and enforces Procedure Description copy
		transformerScript := `// ============================================================================
// UNIVERSAL RADIOLOGY ORDER TRANSFORMER (JSON & HL7 v2 ORM)
// Accepts both JSON payloads from SIMRS and HL7 v2 (ORM^O01) from Mirth/TrakCare
// Saves to SQL dicom_worklist_items and guarantees Procedure Description auto-copy
// ============================================================================

var data;

// 1. Detect if inbound payload is HL7 v2 (starts with MSH) or JSON
if (typeof input === "string" && input.trim().startsWith("MSH")) {
    // Parse HL7 ORM into standardized order JSON
    data = JSON.parse(hl7.toOrderJSON(input));
} else {
    // Parse incoming JSON from SIMRS
    data = typeof input === "string" ? JSON.parse(input) : input;
}

// 2. Ensure Procedure Description is copied across all description attributes
var procDesc = data.procedure_desc || data.procedure_name || data.nama_tindakan || data.study_desc || "Pemeriksaan Radiologi";
data.procedure_desc = procDesc;
data.study_desc = procDesc;
data.req_proc_desc = procDesc;
data.series_desc = procDesc;

// 3. Normalize Date & Time
if (!data.scheduled_date && !data.tgl_periksa) {
    var now = new Date();
    var yyyy = now.getFullYear();
    var mm = String(now.getMonth() + 1).padStart(2, '0');
    var dd = String(now.getDate()).padStart(2, '0');
    data.scheduled_date = yyyy + mm + dd;
}

// 4. Return clean JSON representation for SQL persistence
return JSON.stringify(data);`

		transformerID := uuid.New().String()
		db.Exec("INSERT INTO transformers (id, route_id, source_script, script, inbound_template, outbound_template) VALUES (?, ?, '', ?, '{}', '{}')",
			transformerID, routeID, transformerScript)

		log.Printf("[DB Init] Seeded default Radiology Order & PACS Gateway Channel (%s)", routeID)
	}
}

