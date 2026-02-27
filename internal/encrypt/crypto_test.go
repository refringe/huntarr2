package encrypt

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generating test key: %v", err)
	}
	return key
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	key := testKey(t)
	original := "my-secret-api-key-12345"

	encrypted, err := Encrypt(original, key)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	if encrypted == original {
		t.Error("encrypted value should differ from plaintext")
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}

	if decrypted != original {
		t.Errorf("Decrypt() = %q, want %q", decrypted, original)
	}
}

func TestRandomNonce(t *testing.T) {
	t.Parallel()
	key := testKey(t)
	plaintext := "same-value"

	a, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("first Encrypt() error: %v", err)
	}

	b, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("second Encrypt() error: %v", err)
	}

	if a == b {
		t.Error("two encryptions of the same plaintext should produce different ciphertexts")
	}
}

func TestWrongKeyDecrypt(t *testing.T) {
	t.Parallel()
	keyA := testKey(t)
	keyB := testKey(t)
	plaintext := "secret"

	encrypted, err := Encrypt(plaintext, keyA)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	_, err = Decrypt(encrypted, keyB)
	if err == nil {
		t.Error("Decrypt() with wrong key should return an error")
	}
}

func TestTamperedCiphertext(t *testing.T) {
	t.Parallel()
	key := testKey(t)

	encrypted, err := Encrypt("secret", key)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		t.Fatalf("decoding base64: %v", err)
	}

	raw[len(raw)-1] ^= 0xFF
	tampered := base64.StdEncoding.EncodeToString(raw)

	_, err = Decrypt(tampered, key)
	if err == nil {
		t.Error("Decrypt() with tampered ciphertext should return an error")
	}
}

func TestInvalidKeyLengthEncrypt(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 16, 24, 31, 33, 64} {
		key := make([]byte, size)
		_, err := Encrypt("test", key)
		if err == nil {
			t.Errorf("Encrypt() with %d-byte key should return an error", size)
		}
	}
}

func TestInvalidKeyLengthDecrypt(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 16, 24, 31, 33, 64} {
		key := make([]byte, size)
		_, err := Decrypt("dGVzdA==", key)
		if err == nil {
			t.Errorf("Decrypt() with %d-byte key should return an error", size)
		}
	}
}

func TestDecryptEmptyCiphertext(t *testing.T) {
	t.Parallel()
	key := testKey(t)

	_, err := Decrypt("", key)
	if err == nil {
		t.Error("Decrypt() with empty ciphertext should return an error")
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	t.Parallel()
	key := testKey(t)

	_, err := Decrypt("not-valid-base64!!!", key)
	if err == nil {
		t.Error("Decrypt() with invalid base64 should return an error")
	}
}

func TestEmptyPlaintext(t *testing.T) {
	t.Parallel()
	key := testKey(t)

	encrypted, err := Encrypt("", key)
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	decrypted, err := Decrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}

	if decrypted != "" {
		t.Errorf("Decrypt() = %q, want empty string", decrypted)
	}
}
