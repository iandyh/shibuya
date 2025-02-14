package keys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func GenerateSalt() ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}

func GenerateAPIKey() (string, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}
	salt, err := GenerateSalt()
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	hash.Write(key)
	hash.Write(salt)
	hashedKey := hash.Sum(nil)

	return hex.EncodeToString(hashedKey), nil
}
