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

	// Ensure default admin user exists
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count == 0 {
		hashed, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
		db.Exec("INSERT INTO users (id, username, password_hash, active) VALUES (?, ?, ?, ?)",
			uuid.New().String(), "admin", string(hashed), true)
	}

	return nil
}
