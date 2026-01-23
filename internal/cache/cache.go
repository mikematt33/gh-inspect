package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Cache handles disk-based caching with TTL
type Cache struct {
	baseDir string
	ttl     time.Duration
}

// CacheEntry represents a cached item with metadata
type CacheEntry struct {
	Key       string          `json:"key"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt time.Time       `json:"expires_at"`
}

// New creates a new cache instance
func New(baseDir string, ttl time.Duration) (*Cache, error) {
	if baseDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		baseDir = filepath.Join(homeDir, ".gh-inspect", "cache")
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{
		baseDir: baseDir,
		ttl:     ttl,
	}, nil
}

// Get retrieves a cached value by key
func (c *Cache) Get(key string, value interface{}) (bool, error) {
	cacheFile := c.getCacheFilePath(key)

	// Check if file exists
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // Cache miss
		}
		return false, fmt.Errorf("failed to read cache file: %w", err)
	}

	// Unmarshal cache entry
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		// Invalid cache file, remove it
		_ = os.Remove(cacheFile)
		return false, nil
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		_ = os.Remove(cacheFile)
		return false, nil // Expired
	}

	// Unmarshal the cached data into the value
	if err := json.Unmarshal(entry.Data, value); err != nil {
		return false, fmt.Errorf("failed to unmarshal cached data: %w", err)
	}

	return true, nil
}

// Set stores a value in the cache with TTL
func (c *Cache) Set(key string, value interface{}) error {
	cacheFile := c.getCacheFilePath(key)

	// Ensure the cache directory exists
	if err := os.MkdirAll(filepath.Dir(cacheFile), 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Marshal the value
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// Create cache entry
	entry := CacheEntry{
		Key:       key,
		Data:      data,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(c.ttl),
	}

	// Marshal cache entry
	entryData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	// Write to file
	if err := os.WriteFile(cacheFile, entryData, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Clear removes all cached entries
func (c *Cache) Clear() error {
	return os.RemoveAll(c.baseDir)
}

// Stats returns cache statistics
func (c *Cache) Stats() (int, int64, error) {
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("failed to read cache directory: %w", err)
	}

	var totalSize int64
	validCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		totalSize += info.Size()

		// Check if valid and not expired
		filePath := filepath.Join(c.baseDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var cacheEntry CacheEntry
		if err := json.Unmarshal(data, &cacheEntry); err != nil {
			continue
		}

		if time.Now().Before(cacheEntry.ExpiresAt) {
			validCount++
		}
	}

	return validCount, totalSize, nil
}

// getCacheFilePath generates a cache file path for a given key
func (c *Cache) getCacheFilePath(key string) string {
	// Use SHA256 hash of the key as filename to avoid filesystem issues
	hash := sha256.Sum256([]byte(key))
	filename := hex.EncodeToString(hash[:]) + ".json"
	return filepath.Join(c.baseDir, filename)
}

// GetDefaultCachePath returns the default cache directory path
func GetDefaultCachePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".gh-inspect", "cache"), nil
}
