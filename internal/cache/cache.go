package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"newsfall/internal/model"
)

const version = 1

type diskCache struct {
	Version  int             `json:"version"`
	SavedAt  time.Time       `json:"saved_at"`
	Articles []model.Article `json:"articles"`
}

func Load(path string) ([]model.Article, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read cache: %w", err)
	}
	var stored diskCache
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("parse cache: %w", err)
	}
	if stored.Version != version {
		return nil, fmt.Errorf("unsupported cache version %d", stored.Version)
	}
	return stored.Articles, nil
}

func Save(path string, articles []model.Article) error {
	stored := diskCache{Version: version, SavedAt: time.Now().UTC(), Articles: articles}
	data, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("encode cache: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".newsfall-cache-*.json")
	if err != nil {
		return fmt.Errorf("create temporary cache: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write cache: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close cache: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("replace cache: %w", err)
	}
	return nil
}
