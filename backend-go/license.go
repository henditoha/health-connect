package main

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Embedded Public Key used by the middleware client to verify licenses issued by admin.
const LicensePublicKeyPEM = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA1Hn74M9rL8V2QzW6K0sC
O2uAknx7sXG6qf+U+k5Y5c4q8I7A3H1u9K2b6v+W0e4i7k1n5q9r2u5t8w1x3y4z
A5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6
C7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8
E9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0
A1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2
CwIDAQAB
-----END PUBLIC KEY-----`

type LicenseClaims struct {
	MachineCode string `json:"machine_code"`
	Customer    string `json:"customer"`
	MaxRoutes   int    `json:"max_routes"`
	ExpiresAt   string `json:"expires_at"` // YYYY-MM-DD
	IssuedAt    string `json:"issued_at"`
	Type        string `json:"type"`       // PRODUCTION, TRIAL, ENTERPRISE
}

type LicensePayload struct {
	Claims    LicenseClaims `json:"claims"`
	Signature string        `json:"signature"`
}

type LicenseStatus struct {
	Valid       bool          `json:"valid"`
	IsTrial     bool          `json:"is_trial"`
	StatusText  string        `json:"status_text"` // TRIAL_ACTIVE, TRIAL_EXPIRED, ACTIVE, EXPIRED, INVALID
	Message     string        `json:"message"`
	MachineCode string        `json:"machine_code"`
	FilePath    string        `json:"file_path"`
	Customer    string        `json:"customer"`
	ExpiresAt   string        `json:"expires_at"`
	Type        string        `json:"type"`
	MaxRoutes   int           `json:"max_routes"`
	DaysLeft    int           `json:"days_left"`
	Claims      LicenseClaims `json:"claims"`
}

var (
	currentLicenseMutex sync.RWMutex
	currentLicenseStatus LicenseStatus
)

// GetMachineCode generates a deterministic, stable Machine ID across Windows, macOS, and Linux
func GetMachineCode() string {
	var rawID string
	switch runtime.GOOS {
	case "windows":
		rawID = getWindowsMachineID()
	case "darwin":
		rawID = getDarwinMachineID()
	case "linux":
		rawID = getLinuxMachineID()
	}

	if rawID == "" {
		// Fallback to first non-loopback MAC address + Hostname
		rawID = getNetworkFallbackID()
	}

	h := sha256.New()
	h.Write([]byte("HEALTH-CONNECT-SALT-2026:" + strings.TrimSpace(rawID)))
	sum := hex.EncodeToString(h.Sum(nil))

	// Format like: HC-A1B2-C3D4-E5F6-7890
	upper := strings.ToUpper(sum)
	if len(upper) >= 16 {
		return fmt.Sprintf("HC-%s-%s-%s-%s", upper[0:4], upper[4:8], upper[8:12], upper[12:16])
	}
	return "HC-" + upper[:16]
}

func getWindowsMachineID() string {
	// 1. Try MachineGuid from Registry via powershell or reg
	out, err := exec.Command("reg", "query", `HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid").Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, l := range lines {
			if strings.Contains(l, "MachineGuid") {
				fields := strings.Fields(l)
				if len(fields) >= 3 {
					return fields[len(fields)-1]
				}
			}
		}
	}

	// 2. Fallback to wmic/powershell csproduct UUID
	out, err = exec.Command("powershell", "-NoProfile", "-Command", "(Get-CimInstance -Class Win32_ComputerSystemProduct).UUID").Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 5 {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func getDarwinMachineID() string {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err == nil {
		for _, l := range strings.Split(string(out), "\n") {
			if strings.Contains(l, "IOPlatformUUID") {
				parts := strings.Split(l, "=")
				if len(parts) == 2 {
					return strings.Trim(strings.TrimSpace(parts[1]), `"`)
				}
			}
		}
	}
	return ""
}

func getLinuxMachineID() string {
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		b, err := os.ReadFile(p)
		if err == nil && len(strings.TrimSpace(string(b))) > 0 {
			return strings.TrimSpace(string(b))
		}
	}
	return ""
}

func getNetworkFallbackID() string {
	hostname, _ := os.Hostname()
	mac := "00:00:00:00:00:00"
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp != 0 && !bytes.Equal(iface.HardwareAddr, nil) {
				mac = iface.HardwareAddr.String()
				break
			}
		}
	}
	return hostname + ":" + mac
}

// GetLicenseFilePath returns the canonical platform-specific storage path for license.key
func GetLicenseFilePath() string {
	var dir string
	switch runtime.GOOS {
	case "windows":
		// ProgramData or LocalAppData or Current Working Dir
		dir = os.Getenv("ProgramData")
		if dir == "" {
			dir = os.Getenv("LOCALAPPDATA")
		}
		if dir != "" {
			dir = filepath.Join(dir, "HealthConnect")
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Library", "Application Support", "HealthConnect")
	default:
		// Linux
		dir = "/etc/health-connect"
		// If not root, fallback to ~/.config/health-connect
		if os.Geteuid() != 0 {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, ".config", "health-connect")
		}
	}

	if dir != "" {
		_ = os.MkdirAll(dir, 0755)
		return filepath.Join(dir, "license.key")
	}
	return "license.key"
}

// InitLicenseSystem initializes table, loads license from file/DB, and runs initial validation
func InitLicenseSystem() {
	// Create table in db if not exists
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS app_license (
		id INTEGER PRIMARY KEY,
		license_key TEXT,
		trial_started_at TEXT,
		updated_at TEXT
	)`)
	if err != nil {
		log.Printf("License table init error: %v", err)
	}

	ReloadAndVerifyLicense()
}

// ReloadAndVerifyLicense checks both OS license file and DB, enabling Trial mode if no key is installed
func ReloadAndVerifyLicense() LicenseStatus {
	currentLicenseMutex.Lock()
	defer currentLicenseMutex.Unlock()

	machineCode := GetMachineCode()
	filePath := GetLicenseFilePath()
	status := LicenseStatus{
		Valid:       false,
		MachineCode: machineCode,
		FilePath:    filePath,
	}

	// 1. Try reading from file
	var rawKey string
	if b, err := os.ReadFile(filePath); err == nil {
		rawKey = strings.TrimSpace(string(b))
	}

	// 2. If file is empty, check SQLite
	if rawKey == "" {
		var dbKey string
		row := db.QueryRow("SELECT license_key FROM app_license WHERE id = 1")
		if err := row.Scan(&dbKey); err == nil && dbKey != "" {
			rawKey = strings.TrimSpace(dbKey)
			// Write back to file for multi-location redundancy
			_ = os.WriteFile(filePath, []byte(rawKey), 0644)
		}
	}

	// 3. If no license key installed, activate/evaluate Machine-Bound 14-Day Trial
	if rawKey == "" {
		var trialStartedAt sql.NullString
		_ = db.QueryRow("SELECT trial_started_at FROM app_license WHERE id = 1").Scan(&trialStartedAt)
		var trialStart time.Time
		if !trialStartedAt.Valid || trialStartedAt.String == "" {
			trialStart = time.Now()
			_, _ = db.Exec(`INSERT INTO app_license (id, trial_started_at, updated_at) VALUES (1, ?, ?)
				ON CONFLICT(id) DO UPDATE SET trial_started_at = excluded.trial_started_at`,
				trialStart.Format(time.RFC3339), time.Now().Format(time.RFC3339))
		} else {
			parsedTime, parseErr := time.Parse(time.RFC3339, trialStartedAt.String)
			if parseErr != nil {
				trialStart = time.Now()
			} else {
				trialStart = parsedTime
			}
		}

		trialExp := trialStart.AddDate(0, 0, 14)
		daysLeft := int(time.Until(trialExp).Hours() / 24)
		if daysLeft < 0 {
			daysLeft = 0
		}

		status.IsTrial = true
		status.Type = "TRIAL"
		status.Customer = "Evaluasi Trial / Uji Coba"
		status.ExpiresAt = trialExp.Format("2006-01-02")
		status.MaxRoutes = 10 // Trial 10 channel (baik enable maupun disabled)
		status.DaysLeft = daysLeft
		status.Claims = LicenseClaims{
			MachineCode: machineCode,
			Customer:    status.Customer,
			MaxRoutes:   status.MaxRoutes,
			ExpiresAt:   status.ExpiresAt,
			IssuedAt:    trialStart.Format("2006-01-02"),
			Type:        "TRIAL",
		}

		if time.Now().Before(trialExp) {
			status.Valid = true
			status.StatusText = "TRIAL_ACTIVE"
			status.Message = fmt.Sprintf("Masa evaluasi Trial aktif sampai %s (Tersisa %d hari)", status.ExpiresAt, daysLeft)
		} else {
			status.Valid = false
			status.StatusText = "TRIAL_EXPIRED"
			status.Message = fmt.Sprintf("Masa evaluasi Trial telah berakhir pada %s. Silakan masukkan License Key untuk aktivasi penuh.", status.ExpiresAt)
		}

		currentLicenseStatus = status
		return status
	}

	// 4. Verify cryptographic signature & machine claims
	claims, err := VerifyLicenseKey(rawKey, machineCode)
	if err != nil {
		status.Valid = false
		status.StatusText = "INVALID"
		status.Message = fmt.Sprintf("Lisensi tidak valid: %v", err)
		currentLicenseStatus = status
		return status
	}

	// Calculate remaining days
	expTime, err := time.Parse("2006-01-02", claims.ExpiresAt)
	if err != nil {
		status.Valid = false
		status.StatusText = "INVALID"
		status.Message = "Format tanggal kedaluwarsa lisensi tidak valid."
		currentLicenseStatus = status
		return status
	}

	daysLeft := int(time.Until(expTime).Hours() / 24)
	isExpired := time.Now().After(expTime.Add(24 * time.Hour))
	if daysLeft < 0 {
		daysLeft = 0
	}

	status.Claims = *claims
	status.Customer = claims.Customer
	status.ExpiresAt = claims.ExpiresAt
	status.MaxRoutes = claims.MaxRoutes
	status.DaysLeft = daysLeft
	status.Type = claims.Type
	if status.Type == "" {
		status.Type = "PRODUCTION"
	}
	status.IsTrial = strings.ToUpper(status.Type) == "TRIAL"

	if isExpired {
		status.Valid = false
		if status.IsTrial {
			status.StatusText = "TRIAL_EXPIRED"
			status.Message = fmt.Sprintf("Lisensi Trial telah kedaluwarsa pada %s.", claims.ExpiresAt)
		} else {
			status.StatusText = "EXPIRED"
			status.Message = fmt.Sprintf("Lisensi telah kedaluwarsa pada %s.", claims.ExpiresAt)
		}
	} else {
		status.Valid = true
		if status.IsTrial {
			status.StatusText = "TRIAL_ACTIVE"
			status.Message = fmt.Sprintf("Lisensi Trial resmi aktif untuk %s (Tersisa %d hari sampai %s)", claims.Customer, daysLeft, claims.ExpiresAt)
		} else {
			status.StatusText = "ACTIVE"
			status.Message = fmt.Sprintf("Lisensi resmi aktif untuk %s (Tersisa %d hari sampai %s)", claims.Customer, daysLeft, claims.ExpiresAt)
		}
	}

	currentLicenseStatus = status
	return status
}

// VerifyLicenseKey verifies RSA signature and machine binding
func VerifyLicenseKey(encodedLicense string, currentMachineCode string) (*LicenseClaims, error) {
	jsonBytes, err := base64.StdEncoding.DecodeString(encodedLicense)
	if err != nil {
		// Try raw JSON if user pasted unencoded JSON
		jsonBytes = []byte(encodedLicense)
	}

	var payload LicensePayload
	if err := json.Unmarshal(jsonBytes, &payload); err != nil {
		return nil, errors.New("format payload lisensi rusak")
	}

	// Check machine binding
	if payload.Claims.MachineCode != "*" && payload.Claims.MachineCode != currentMachineCode {
		return nil, fmt.Errorf("kode mesin tidak cocok (lisensi diterbitkan untuk %s, mesin Anda: %s)", payload.Claims.MachineCode, currentMachineCode)
	}

	// Verify RSA signature
	claimsBytes, err := json.Marshal(payload.Claims)
	if err != nil {
		return nil, err
	}

	sigBytes, err := base64.StdEncoding.DecodeString(payload.Signature)
	if err != nil {
		return nil, errors.New("signature lisensi rusak")
	}

	block, _ := pem.Decode([]byte(LicensePublicKeyPEM))
	if block == nil {
		return nil, errors.New("gagal memuat public key verifikasi")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("tipe public key tidak didukung")
	}

	hashed := sha256.Sum256(claimsBytes)
	err = rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, hashed[:], sigBytes)
	if err != nil {
		return nil, errors.New("tanda tangan digital lisensi tidak sah (tampered)")
	}

	return &payload.Claims, nil
}

// SaveLicenseKey writes license to disk and database
func SaveLicenseKey(rawKey string) error {
	rawKey = strings.TrimSpace(rawKey)
	machineCode := GetMachineCode()

	// Verify before saving
	_, err := VerifyLicenseKey(rawKey, machineCode)
	if err != nil {
		return err
	}

	// 1. Write to OS File
	filePath := GetLicenseFilePath()
	if err := os.WriteFile(filePath, []byte(rawKey), 0644); err != nil {
		log.Printf("Warning saving license file: %v", err)
	}

	// 2. Write to SQLite
	_, err = db.Exec(`INSERT INTO app_license (id, license_key, updated_at) VALUES (1, ?, ?)
		ON CONFLICT(id) DO UPDATE SET license_key = excluded.license_key, updated_at = excluded.updated_at`,
		rawKey, time.Now().Format(time.RFC3339))

	ReloadAndVerifyLicense()
	return err
}

// CheckCanCreateRoute checks if total registered routes (both enabled and disabled) exceed licensed limit
func CheckCanCreateRoute() error {
	currentLicenseMutex.RLock()
	lic := currentLicenseStatus
	currentLicenseMutex.RUnlock()

	if !lic.Valid {
		if lic.IsTrial {
			return fmt.Errorf("Masa evaluasi Trial telah berakhir pada %s. Silakan masukkan License Key untuk menambah channel.", lic.ExpiresAt)
		}
		return errors.New("Aplikasi membutuhkan lisensi aktif untuk menambah channel.")
	}

	maxRoutes := lic.MaxRoutes
	if lic.Claims.MaxRoutes > 0 {
		maxRoutes = lic.Claims.MaxRoutes
	}

	if maxRoutes > 0 {
		var totalCount int
		// Baik enabled maupun disabled dihitung ke total kuota
		row := db.QueryRow("SELECT COUNT(*) FROM routes")
		if err := row.Scan(&totalCount); err != nil {
			return err
		}

		if totalCount >= maxRoutes {
			if lic.IsTrial {
				return fmt.Errorf("Batas kuota Trial tercapai (maksimal %d channel terdaftar, baik enable maupun disabled). Channel tidak dapat ditambah lagi.", maxRoutes)
			}
			return fmt.Errorf("Batas kuota total channel tercapai (%d / %d channel). Upgrade lisensi untuk menambah channel.", totalCount, maxRoutes)
		}
	}
	return nil
}

// CheckCanDeleteRoute enforces that channels cannot be deleted during Trial mode
func CheckCanDeleteRoute() error {
	currentLicenseMutex.RLock()
	lic := currentLicenseStatus
	currentLicenseMutex.RUnlock()

	if lic.IsTrial {
		return errors.New("Channel tidak dapat dihapus selama mode evaluasi Trial.")
	}
	return nil
}

// CheckMaxRouteLimit checks if active channels exceed licensed limit
func CheckMaxRouteLimit() error {
	currentLicenseMutex.RLock()
	lic := currentLicenseStatus
	currentLicenseMutex.RUnlock()

	if !lic.Valid {
		if lic.IsTrial {
			return fmt.Errorf("Masa evaluasi Trial telah berakhir pada %s. Silakan masukkan License Key untuk mengaktifkan channel.", lic.ExpiresAt)
		}
		return errors.New("Aplikasi membutuhkan lisensi aktif untuk mengaktifkan channel.")
	}

	// Dalam mode Trial, hingga 10 channel yang dibuat bebas di-enable atau di-disable
	if lic.IsTrial {
		return nil
	}

	maxRoutes := lic.MaxRoutes
	if lic.Claims.MaxRoutes > 0 {
		maxRoutes = lic.Claims.MaxRoutes
	}

	if maxRoutes > 0 {
		var activeCount int
		row := db.QueryRow("SELECT COUNT(*) FROM routes WHERE active = 1")
		if err := row.Scan(&activeCount); err != nil {
			return err
		}

		if activeCount >= maxRoutes {
			return fmt.Errorf("Batas kuota channel aktif telah tercapai (%d / %d channel). Upgrade lisensi untuk menambah channel.", activeCount, maxRoutes)
		}
	}

	return nil
}

// -------------------------------------------------------------------------
// GIN API HANDLERS
// -------------------------------------------------------------------------

func getLicenseInfo(c *gin.Context) {
	currentLicenseMutex.RLock()
	status := currentLicenseStatus
	currentLicenseMutex.RUnlock()

	c.JSON(200, status)
}

func activateLicense(c *gin.Context) {
	var input struct {
		LicenseKey string `json:"license_key"`
	}
	if err := c.BindJSON(&input); err != nil || strings.TrimSpace(input.LicenseKey) == "" {
		c.JSON(400, gin.H{"error": "License key wajib diisi"})
		return
	}

	if err := SaveLicenseKey(input.LicenseKey); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	currentLicenseMutex.RLock()
	status := currentLicenseStatus
	currentLicenseMutex.RUnlock()

	c.JSON(200, gin.H{
		"message": "Lisensi berhasil diaktivasi!",
		"status":  status,
	})
}
