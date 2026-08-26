package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
)

const apiKeyPrefix = "cts_"

func CreateAPIKey(name string) (plain string, prefix string, hash []byte, err error) {
	if strings.TrimSpace(name) == "" {
		return "", "", nil, fmt.Errorf("key name cannot be empty")
	}
	value, err := randomValue()
	if err != nil {
		return "", "", nil, err
	}
	plain = apiKeyPrefix + value
	prefix = plain[:min(len(plain), 12)]
	return plain, prefix, HashSecret(plain), nil
}

func NewSession() (plain string, hash []byte, err error) {
	plain, err = randomValue()
	if err != nil {
		return "", nil, err
	}
	return plain, HashSecret(plain), nil
}

func randomValue() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func HashSecret(plain string) []byte {
	hash := sha256.Sum256([]byte(plain))
	return hash[:]
}

func VerifySecret(candidate string, stored []byte) bool {
	candidateHash := HashSecret(candidate)
	return subtle.ConstantTimeCompare(candidateHash, stored) == 1
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
