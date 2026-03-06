package main

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// VersionCache はパッケージIDごとのバージョン情報をメモリとディスクに永続化するキャッシュ。
// TTLなし・常に有効。refreshAll() によって定期的に上書きされる。
type VersionCache struct {
	mu        sync.RWMutex
	entries   map[string][]Version
	cacheFile string
}

// NewVersionCache は新しい VersionCache を返す。
// cacheFile が空の場合はディスクキャッシュを使用しない。
// 起動時にディスクからロードする。
func NewVersionCache(cacheFile string) *VersionCache {
	c := &VersionCache{
		entries:   make(map[string][]Version),
		cacheFile: cacheFile,
	}
	if cacheFile != "" {
		if err := c.loadDiskCache(); err != nil {
			slog.Warn("failed to load version disk cache", "file", cacheFile, "error", err)
		}
	}
	return c
}

func (c *VersionCache) Get(id string) ([]Version, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.entries[id]
	return v, ok
}

func (c *VersionCache) Set(id string, versions []Version) {
	c.mu.Lock()
	c.entries[id] = versions
	c.mu.Unlock()

	if c.cacheFile != "" {
		if err := c.saveDiskCache(); err != nil {
			slog.Warn("failed to save version disk cache", "file", c.cacheFile, "error", err)
		}
	}
}

// All は SHA256 Seed 用にすべてのエントリのスナップショットを返す。
func (c *VersionCache) All() map[string][]Version {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make(map[string][]Version, len(c.entries))
	for k, v := range c.entries {
		result[k] = v
	}
	return result
}

func (c *VersionCache) loadDiskCache() error {
	data, err := os.ReadFile(c.cacheFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var loaded map[string][]Version
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range loaded {
		c.entries[k] = v
	}
	return nil
}

func (c *VersionCache) saveDiskCache() error {
	c.mu.RLock()
	data, err := json.Marshal(c.entries)
	c.mu.RUnlock()
	if err != nil {
		return err
	}

	dir := filepath.Dir(c.cacheFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "versioncache-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, c.cacheFile)
}
