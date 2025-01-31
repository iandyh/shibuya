package keys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func GenerateAPIKey() (string, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}

	salt := make([]byte, 16)
	_, err = rand.Read(salt)
	if err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := sha256.New()
	hash.Write(key)
	hash.Write(salt)
	hashedKey := hash.Sum(nil)

	return hex.EncodeToString(hashedKey), nil
}
