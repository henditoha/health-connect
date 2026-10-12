package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const bannerSeparator = "======================================================="

type LicenseClaims struct {
	MachineCode string `json:"machine_code"`
	Customer    string `json:"customer"`
	MaxRoutes   int    `json:"max_routes"`
	ExpiresAt   string `json:"expires_at"` // YYYY-MM-DD
	IssuedAt    string `json:"issued_at"`
	Type        string `json:"type"` // PRODUCTION, TRIAL, ENTERPRISE
}

type LicensePayload struct {
	Claims    LicenseClaims `json:"claims"`
	Signature string        `json:"signature"`
}

func loadPrivateKeyBytes(customPath string) ([]byte, error) {
	if customPath != "" {
		if b, err := os.ReadFile(customPath); err == nil {
			return b, nil
		}
	}
	if envPath := os.Getenv("ADMIN_PRIVATE_KEY_PATH"); envPath != "" {
		if b, err := os.ReadFile(envPath); err == nil {
			return b, nil
		}
	}
	if envKey := os.Getenv("ADMIN_PRIVATE_KEY"); envKey != "" {
		return []byte(envKey), nil
	}

	// Default fallback locations
	candidates := []string{
		"admin_private.pem",
		"tools/license-gen/admin_private.pem",
		"backend-go/tools/license-gen/admin_private.pem",
	}
	for _, p := range candidates {
		if b, err := os.ReadFile(p); err == nil {
			return b, nil
		}
	}

	return nil, fmt.Errorf("file private key tidak ditemukan di '%s' atau path default (admin_private.pem)", customPath)
}

func main() {
	machineCode := flag.String("machine", "", "Kode Mesin klien (contoh: HC-A1B2-C3D4-E5F6-7890 atau * untuk semua)")
	customer := flag.String("customer", "RSUD Sejahtera", "Nama Rumah Sakit / Faskes Klien")
	maxRoutes := flag.Int("max-routes", 10, "Batas jumlah route aktif (0 = unlimited)")
	days := flag.Int("days", 365, "Masa aktif dalam hari")
	licType := flag.String("type", "PRODUCTION", "Tipe lisensi (TRIAL, PRODUCTION, ENTERPRISE)")
	output := flag.String("out", "license.key", "Nama file output lisensi")
	keyPath := flag.String("key", "admin_private.pem", "Path ke file RSA private key PEM")
	flag.Parse()

	if *machineCode == "" {
		fmt.Println("❌ Error: Parameter -machine wajib diisi!")
		fmt.Println("Contoh penggunaan:")
		fmt.Println("  go run ./tools/license-gen -machine HC-XXXX-XXXX-XXXX-XXXX -customer \"RSUP Harapan\" -days 365 -max-routes 15")
		os.Exit(1)
	}

	expDate := time.Now().AddDate(0, 0, *days).Format("2006-01-02")
	issuedDate := time.Now().Format("2006-01-02")

	claims := LicenseClaims{
		MachineCode: *machineCode,
		Customer:    *customer,
		MaxRoutes:   *maxRoutes,
		ExpiresAt:   expDate,
		IssuedAt:    issuedDate,
		Type:        *licType,
	}

	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		fmt.Printf("Error serializing claims: %v\n", err)
		os.Exit(1)
	}

	keyBytes, err := loadPrivateKeyBytes(*keyPath)
	if err != nil {
		fmt.Printf("❌ Error memuat private key: %v\n", err)
		fmt.Println("Silakan sediakan file private key dengan flag -key <path/ke/admin_private.pem>")
		os.Exit(1)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		fmt.Println("Error decoding private key PEM")
		os.Exit(1)
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		fmt.Printf("Error parsing private key: %v\n", err)
		os.Exit(1)
	}

	hashed := sha256.Sum256(claimsBytes)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, hashed[:])
	if err != nil {
		fmt.Printf("Error signing license: %v\n", err)
		os.Exit(1)
	}

	payload := LicensePayload{
		Claims:    claims,
		Signature: base64.StdEncoding.EncodeToString(signature),
	}

	payloadJSON, _ := json.Marshal(payload)
	licenseBase64 := base64.StdEncoding.EncodeToString(payloadJSON)

	// Write to file
	err = os.WriteFile(*output, []byte(licenseBase64), 0644)
	if err != nil {
		fmt.Printf("Error saving license file: %v\n", err)
		os.Exit(1)
	}

	absPath, _ := filepath.Abs(*output)
	fmt.Println(bannerSeparator)
	fmt.Println("✅ LISENSI BERHASIL DIBUAT DENGAN TANDA TANGAN RSA")
	fmt.Println(bannerSeparator)
	fmt.Printf("Klien        : %s\n", claims.Customer)
	fmt.Printf("Kode Mesin   : %s\n", claims.MachineCode)
	fmt.Printf("Tipe         : %s\n", claims.Type)
	fmt.Printf("Batas Route  : %d route (0 = unlimited)\n", claims.MaxRoutes)
	fmt.Printf("Berlaku s/d  : %s (%d hari)\n", claims.ExpiresAt, *days)
	fmt.Printf("File Lisensi : %s\n", absPath)
	fmt.Println("-------------------------------------------------------")
	fmt.Println("Kunci Lisensi (Salin string berikut jika input manual):")
	fmt.Println(licenseBase64)
	fmt.Println(bannerSeparator)
}
