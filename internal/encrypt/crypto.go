// Package encrypt provides AES-256-GCM encryption and decryption for sensitive
// values stored at rest, such as API keys.
package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

const keyLength = 32

// Encrypt encrypts plaintext using AES-256-GCM with the provided
// 32-byte key. A random nonce is prepended to the ciphertext, and the
// result is returned as a base64-encoded string.
func Encrypt(plaintext string, key []byte) (string, error) {
	if len(key) != keyLength {
		return "", fmt.Errorf("encryption key must be %d bytes, got %d", keyLength, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt: it base64-decodes the input, extracts the nonce,
// and decrypts the remaining ciphertext using AES-256-GCM with the provided
// 32-byte key.
func Decrypt(ciphertext string, key []byte) (string, error) {
	if len(key) != keyLength {
		return "", fmt.Errorf("decryption key must be %d bytes, got %d", keyLength, len(key))
	}

	if ciphertext == "" {
		return "", fmt.Errorf("ciphertext is empty")
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decoding base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, encrypted := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}

	return string(plaintext), nil
}
