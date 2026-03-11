package models

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const encPrefix = "enc:"

var encryptionKey []byte

// SensitiveKeys is the set of configuration keys whose values are encrypted at rest.
var SensitiveKeys = map[string]bool{
	"zendesk_api_key": true,
	"slack_bot_token": true,
	"slack_app_token": true,
}

// InitEncryption derives a 32-byte AES key from the session key using HKDF.
// Must be called before any encrypted config operations.
// The salt incorporates the session key hash to ensure different instances
// with different keys produce different encryption keys.
func InitEncryption(sessionKey []byte) {
	keyHash := sha256.Sum256(sessionKey)
	salt := append([]byte("ticketpulse-config-encryption-"), keyHash[:8]...)
	hkdfReader := hkdf.New(sha256.New, sessionKey, salt, []byte("aes-256-gcm"))
	encryptionKey = make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, encryptionKey); err != nil {
		panic(fmt.Sprintf("failed to derive encryption key: %v", err))
	}
}

var ErrEncryptionNotInitialized = fmt.Errorf("encryption not initialized: call InitEncryption before storing sensitive values")

func encrypt(plaintext string) (string, error) {
	if len(encryptionKey) == 0 {
		return "", ErrEncryptionNotInitialized
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, encPrefix) {
		if len(encryptionKey) == 0 {
			return ciphertext, nil
		}
		return ciphertext, nil
	}

	if len(encryptionKey) == 0 {
		return "", ErrEncryptionNotInitialized
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext[len(encPrefix):])
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
