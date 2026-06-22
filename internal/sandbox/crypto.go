package sandbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	encryptionKey []byte
	keyOnce       sync.Once
	keyErr        error
)

func getEncryptionKey(configDir string) ([]byte, error) {
	keyOnce.Do(func() {
		keyPath := filepath.Join(configDir, ".enc_key")
		key, err := os.ReadFile(keyPath)
		if err == nil && len(key) == 32 {
			encryptionKey = key
			return
		}

		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			keyErr = fmt.Errorf("generate key: %w", err)
			return
		}

		if err := os.MkdirAll(configDir, 0755); err != nil {
			keyErr = fmt.Errorf("create config dir: %w", err)
			return
		}
		if err := os.WriteFile(keyPath, key, 0600); err != nil {
			keyErr = fmt.Errorf("write key: %w", err)
			return
		}
		encryptionKey = key
	})
	return encryptionKey, keyErr
}

func EncryptAPIKey(plaintext string, configDir string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	key, err := getEncryptionKey(configDir)
	if err != nil {
		return plaintext, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return plaintext, nil
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return plaintext, nil
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return "enc:" + base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptAPIKey(encrypted string, configDir string) (string, error) {
	if encrypted == "" {
		return "", nil
	}

	if !strings.HasPrefix(encrypted, "enc:") {
		return encrypted, nil
	}

	key, err := getEncryptionKey(configDir)
	if err != nil {
		return "", fmt.Errorf("get encryption key: %w", err)
	}

	data, err := base64.StdEncoding.DecodeString(encrypted[4:])
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}
