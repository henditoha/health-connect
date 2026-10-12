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

// Default Embedded Private Key for Generator Tool (Keep secure, used by admin)
const AdminPrivateKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA1Hn74M9rL8V2QzW6K0sCO2uAknx7sXG6qf+U+k5Y5c4q8I7A
3H1u9K2b6v+W0e4i7k1n5q9r2u5t8w1x3y4zA5b6c7d8e9f0a1b2c3d4e5f6a7b8
c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6C7d8e9f0a1b2c3d4e5f6a7b8c9d0
e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8E9f0a1b2c3d4e5f6a7b8c9d0e1f2
a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0A1b2c3d4e5f6a7b8c9d0e1f2a3b4
c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2CwIDAQABAoIBAQCpE2lq5m8eN4Z0
yqXkZ6kU5b9q+w8l1k2m3n4p5r6s7t8u9v0w1x2y3z4a5b6c7d8e9f0a1b2c3d4e
5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a
7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c
9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e
1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0AECgYEA9gT7p3m6q2v9x1y4
z5a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f9a8
b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d1e0f9a8b7c6
d5e4f3a2b1c0d9e8f7a6CgYEA3Hn74M9rL8V2QzW6K0sCO2uAknx7sXG6qf+U+k5Y
5c4q8I7A3H1u9K2b6v+W0e4i7k1n5q9r2u5t8w1x3y4zA5b6c7d8e9f0a1b2c3d4
e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6
CgYEA5v6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4
a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6
c7d8e9f0a1b2c3d4e5f6CgYEA7k1n5q9r2u5t8w1x3y4zA5b6c7d8e9f0a1b2c3d4
e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6
a7b8c9d0e1f2a3b4c5d6e7f8CgEBAI7A3H1u9K2b6v+W0e4i7k1n5q9r2u5t8w1x
3y4zA5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4
a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6
-----END RSA PRIVATE KEY-----`

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

func main() {
	machineCode := flag.String("machine", "", "Kode Mesin klien (contoh: HC-A1B2-C3D4-E5F6-7890 atau * untuk semua)")
	customer := flag.String("customer", "RSUD Sejahtera", "Nama Rumah Sakit / Faskes Klien")
	maxRoutes := flag.Int("max-routes", 10, "Batas jumlah route aktif (0 = unlimited)")
	days := flag.Int("days", 365, "Masa aktif dalam hari")
	licType := flag.String("type", "PRODUCTION", "Tipe lisensi (TRIAL, PRODUCTION, ENTERPRISE)")
	output := flag.String("out", "license.key", "Nama file output lisensi")
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

	block, _ := pem.Decode([]byte(AdminPrivateKeyPEM))
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
	fmt.Println("=======================================================")
	fmt.Println("✅ LISENSI BERHASIL DIBUAT DENGAN TANDA TANGAN RSA")
	fmt.Println("=======================================================")
	fmt.Printf("Klien        : %s\n", claims.Customer)
	fmt.Printf("Kode Mesin   : %s\n", claims.MachineCode)
	fmt.Printf("Tipe         : %s\n", claims.Type)
	fmt.Printf("Batas Route  : %d route (0 = unlimited)\n", claims.MaxRoutes)
	fmt.Printf("Berlaku s/d  : %s (%d hari)\n", claims.ExpiresAt, *days)
	fmt.Printf("File Lisensi : %s\n", absPath)
	fmt.Println("-------------------------------------------------------")
	fmt.Println("Kunci Lisensi (Salin string berikut jika input manual):")
	fmt.Println(licenseBase64)
	fmt.Println("=======================================================")
}
