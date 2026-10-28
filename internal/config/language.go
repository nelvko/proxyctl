package config

import (
	"context"
	"errors"
)

// LoadLanguage isolates an optional presentation preference from core settings.
func LoadLanguage() (string, error) {
	v, _, err := readConfig()
	if err != nil || v == nil {
		return "", err
	}
	return decodeLanguage(v.Get("language"))
}

func decodeLanguage(raw any) (string, error) {
	if raw == nil {
		return "", nil
	}
	if value, ok := raw.(string); ok {
		return value, nil
	}
	return "", errors.New("language must be a string; using system/default language")
}

// SaveLanguage preserves state changed by another CLI while the TUI was open.
func SaveLanguage(ctx context.Context, language string) error {
	unlock, err := Lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	cfg, err := Load()
	if err != nil {
		return err
	}
	cfg.Language = language
	return Save(cfg)
}
