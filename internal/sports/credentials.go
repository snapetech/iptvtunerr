package sports

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const maxAPIKeyLength = 512

var (
	ErrInvalidAPIKey = errors.New("invalid API-Sports key")
	ErrAPIKeyPath    = errors.New("API-Sports key file is not configured")
)

func ValidateAPIKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" || len(key) > maxAPIKeyLength || strings.IndexFunc(key, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r)
	}) >= 0 {
		return "", ErrInvalidAPIKey
	}
	return key, nil
}

func LoadAPIKey(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("stat API-Sports key file: %w", err)
	}
	if info.Size() > maxAPIKeyLength+1 {
		return "", ErrInvalidAPIKey
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read API-Sports key file: %w", err)
	}
	key, err := ValidateAPIKey(string(raw))
	if err != nil {
		return "", err
	}
	return key, nil
}

func SaveAPIKey(path, raw string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return ErrAPIKeyPath
	}
	key, err := ValidateAPIKey(raw)
	if err != nil {
		return err
	}
	dir := filepath.Dir(filepath.Clean(path))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create API-Sports key directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".sports-api-key-*.tmp")
	if err != nil {
		return fmt.Errorf("create API-Sports key temp file: %w", err)
	}
	tmpName := tmp.Name()
	_, writeErr := tmp.WriteString(key + "\n")
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(tmpName)
		if writeErr != nil {
			return fmt.Errorf("write API-Sports key file: %w", writeErr)
		}
		return fmt.Errorf("close API-Sports key file: %w", closeErr)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("secure API-Sports key file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("save API-Sports key file: %w", err)
	}
	return nil
}

func RemoveAPIKey(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return ErrAPIKeyPath
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove API-Sports key file: %w", err)
	}
	return nil
}
