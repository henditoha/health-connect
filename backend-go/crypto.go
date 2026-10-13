package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const secretPrefix = "enc:v1:"

var authBearerRegex = regexp.MustCompile(`(?i)(Bearer\s+)([A-Za-z0-9_\-\.]{10,})`)

// getMasterKey derives a 32-byte key for AES-256 using machine ID + optional env/config salt
func getMasterKey() []byte {
	machineCode := GetMachineCode()
	appSalt := os.Getenv("APP_ENCRYPTION_KEY")
	if appSalt == "" {
		appSalt = "HEALTH-CONNECT-SECURE-KEY-2026"
	}
	h := sha256.New()
	h.Write([]byte(machineCode + ":" + appSalt))
	return h.Sum(nil)
}

// EncryptSecret encrypts plaintext using AES-256-GCM and returns a prefixed base64 string
func EncryptSecret(plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}
	if strings.HasPrefix(plainText, secretPrefix) {
		return plainText, nil // already encrypted
	}

	key := getMasterKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("cipher init error: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm init error: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce generation error: %w", err)
	}

	cipherText := gcm.Seal(nonce, nonce, []byte(plainText), nil)
	return secretPrefix + base64.StdEncoding.EncodeToString(cipherText), nil
}

// DecryptSecret decrypts a prefixed base64 string. If not prefixed, returns plainText as fallback.
func DecryptSecret(cipherText string) (string, error) {
	if cipherText == "" {
		return "", nil
	}
	if !strings.HasPrefix(cipherText, secretPrefix) {
		return cipherText, nil // unencrypted plain text fallback
	}

	rawB64 := strings.TrimPrefix(cipherText, secretPrefix)
	data, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}

	key := getMasterKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("cipher init error: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm init error: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, actualCipher := data[:nonceSize], data[nonceSize:]
	plainBytes, err := gcm.Open(nil, nonce, actualCipher, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt error: %w", err)
	}

	return string(plainBytes), nil
}

// SanitizeLogText removes or masks sensitive credentials (like Bearer tokens) from logged text
func SanitizeLogText(text string) string {
	if text == "" {
		return ""
	}
	// Mask Bearer tokens: Bearer <token> -> Bearer ey...[REDACTED]
	return authBearerRegex.ReplaceAllStringFunc(text, func(m string) string {
		parts := authBearerRegex.FindStringSubmatch(m)
		if len(parts) >= 3 {
			token := parts[2]
			if len(token) > 8 {
				return parts[1] + token[:4] + "...[REDACTED]"
			}
			return parts[1] + "[REDACTED]"
		}
		return m
	})
}
