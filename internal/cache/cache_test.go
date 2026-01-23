package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	// Test with custom directory
	tmpDir := t.TempDir()
	c, err := New(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	if c.baseDir != tmpDir {
		t.Errorf("Expected baseDir %s, got %s", tmpDir, c.baseDir)
	}
	if c.ttl != 24*time.Hour {
		t.Errorf("Expected TTL 24h, got %v", c.ttl)
	}

	// Test with empty directory (should use default)
	c2, err := New("", 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache with default dir: %v", err)
	}
	if c2.baseDir == "" {
		t.Error("Expected non-empty baseDir for default cache")
	}
}

func TestSetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := New(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Test data
	testData := map[string]interface{}{
		"name":  "test",
		"count": 42,
		"items": []string{"a", "b", "c"},
	}

	// Set cache entry
	err = c.Set("test-key", testData)
	if err != nil {
		t.Fatalf("Failed to set cache entry: %v", err)
	}

	// Get cache entry
	var retrieved map[string]interface{}
	found, err := c.Get("test-key", &retrieved)
	if err != nil {
		t.Fatalf("Failed to get cache entry: %v", err)
	}
	if !found {
		t.Error("Expected cache hit, got miss")
	}

	// Verify data
	if retrieved["name"] != "test" {
		t.Errorf("Expected name 'test', got %v", retrieved["name"])
	}
	if retrieved["count"].(float64) != 42 {
		t.Errorf("Expected count 42, got %v", retrieved["count"])
	}
}

func TestGetCacheMiss(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := New(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	var data interface{}
	found, err := c.Get("nonexistent-key", &data)
	if err != nil {
		t.Fatalf("Unexpected error on cache miss: %v", err)
	}
	if found {
		t.Error("Expected cache miss, got hit")
	}
}

func TestTTLExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	// Create cache with very short TTL
	c, err := New(tmpDir, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	testData := "test-value"
	err = c.Set("test-key", testData)
	if err != nil {
		t.Fatalf("Failed to set cache entry: %v", err)
	}

	// Should be available immediately
	var retrieved string
	found, err := c.Get("test-key", &retrieved)
	if err != nil {
		t.Fatalf("Failed to get cache entry: %v", err)
	}
	if !found {
		t.Error("Expected cache hit")
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should now be expired
	found, err = c.Get("test-key", &retrieved)
	if err != nil {
		t.Fatalf("Unexpected error on expired entry: %v", err)
	}
	if found {
		t.Error("Expected cache miss due to expiration")
	}
}

func TestInvalidCacheEntry(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := New(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Write invalid JSON to cache file
	cacheFile := c.getCacheFilePath("test-key")
	err = os.MkdirAll(filepath.Dir(cacheFile), 0755)
	if err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}
	err = os.WriteFile(cacheFile, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid cache file: %v", err)
	}

	// Should handle gracefully
	var data interface{}
	found, err := c.Get("test-key", &data)
	if err != nil {
		t.Fatalf("Unexpected error on invalid cache entry: %v", err)
	}
	if found {
		t.Error("Expected cache miss for invalid entry")
	}

	// Invalid file should be removed
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Error("Expected invalid cache file to be removed")
	}
}

func TestClear(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := New(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Add multiple cache entries
	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		err = c.Set(key, i)
		if err != nil {
			t.Fatalf("Failed to set cache entry: %v", err)
		}
	}

	// Verify entries exist
	validCount, _, err := c.Stats()
	if err != nil {
		t.Fatalf("Failed to get cache stats: %v", err)
	}
	if validCount != 5 {
		t.Errorf("Expected 5 cache entries, got %d", validCount)
	}

	// Clear cache
	err = c.Clear()
	if err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}

	// Verify cache is empty
	if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
		// Directory might still exist, check if it's empty
		entries, _ := os.ReadDir(tmpDir)
		if len(entries) > 0 {
			t.Errorf("Expected empty cache directory, got %d entries", len(entries))
		}
	}
}

func TestStats(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := New(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Initially should be empty
	validCount, totalSize, err := c.Stats()
	if err != nil {
		t.Fatalf("Failed to get cache stats: %v", err)
	}
	if validCount != 0 {
		t.Errorf("Expected 0 valid entries, got %d", validCount)
	}
	if totalSize != 0 {
		t.Errorf("Expected 0 total size, got %d", totalSize)
	}

	// Add entries
	testData := map[string]string{"key": "value"}
	for i := 0; i < 3; i++ {
		key := string(rune('a' + i))
		err = c.Set(key, testData)
		if err != nil {
			t.Fatalf("Failed to set cache entry: %v", err)
		}
	}

	// Check stats
	validCount, totalSize, err = c.Stats()
	if err != nil {
		t.Fatalf("Failed to get cache stats: %v", err)
	}
	if validCount != 3 {
		t.Errorf("Expected 3 valid entries, got %d", validCount)
	}
	if totalSize == 0 {
		t.Error("Expected non-zero total size")
	}
}

func TestStatsWithExpiredEntries(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := New(tmpDir, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Add an entry
	err = c.Set("test-key", "test-value")
	if err != nil {
		t.Fatalf("Failed to set cache entry: %v", err)
	}

	// Should have 1 valid entry
	validCount, _, err := c.Stats()
	if err != nil {
		t.Fatalf("Failed to get cache stats: %v", err)
	}
	if validCount != 1 {
		t.Errorf("Expected 1 valid entry, got %d", validCount)
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Should now have 0 valid entries (but file still exists)
	validCount, totalSize, err := c.Stats()
	if err != nil {
		t.Fatalf("Failed to get cache stats after expiration: %v", err)
	}
	if validCount != 0 {
		t.Errorf("Expected 0 valid entries after expiration, got %d", validCount)
	}
	if totalSize == 0 {
		t.Error("Expected non-zero total size (file still exists)")
	}
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := New(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Simulate concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			key := string(rune('a' + idx))
			err := c.Set(key, idx)
			if err != nil {
				t.Errorf("Failed to set cache entry %d: %v", idx, err)
			}
			done <- true
		}(i)
	}

	// Wait for all writes to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all entries are accessible
	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		var value int
		found, err := c.Get(key, &value)
		if err != nil {
			t.Errorf("Failed to get cache entry %d: %v", i, err)
		}
		if !found {
			t.Errorf("Expected to find cache entry for key %s", key)
		}
		if value != i {
			t.Errorf("Expected value %d, got %d", i, value)
		}
	}
}

func TestGetDefaultCachePath(t *testing.T) {
	path, err := GetDefaultCachePath()
	if err != nil {
		t.Fatalf("Failed to get default cache path: %v", err)
	}
	if path == "" {
		t.Error("Expected non-empty default cache path")
	}
	if !filepath.IsAbs(path) {
		t.Error("Expected absolute path")
	}
}

func TestCacheWithComplexData(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := New(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Complex nested structure
	type TestStruct struct {
		Name   string
		Count  int
		Items  []string
		Nested map[string]interface{}
	}

	testData := TestStruct{
		Name:  "complex",
		Count: 100,
		Items: []string{"x", "y", "z"},
		Nested: map[string]interface{}{
			"key1": "value1",
			"key2": 42,
			"key3": []int{1, 2, 3},
		},
	}

	// Set and get
	err = c.Set("complex-key", testData)
	if err != nil {
		t.Fatalf("Failed to set complex cache entry: %v", err)
	}

	var retrieved TestStruct
	found, err := c.Get("complex-key", &retrieved)
	if err != nil {
		t.Fatalf("Failed to get complex cache entry: %v", err)
	}
	if !found {
		t.Error("Expected cache hit")
	}

	// Verify structure
	if retrieved.Name != testData.Name {
		t.Errorf("Expected name %s, got %s", testData.Name, retrieved.Name)
	}
	if retrieved.Count != testData.Count {
		t.Errorf("Expected count %d, got %d", testData.Count, retrieved.Count)
	}
	if len(retrieved.Items) != len(testData.Items) {
		t.Errorf("Expected %d items, got %d", len(testData.Items), len(retrieved.Items))
	}
}

func TestCacheFilePathHashing(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := New(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Different keys should produce different file paths
	path1 := c.getCacheFilePath("key1")
	path2 := c.getCacheFilePath("key2")

	if path1 == path2 {
		t.Error("Expected different cache file paths for different keys")
	}

	// Same key should always produce same path
	path1a := c.getCacheFilePath("key1")
	if path1 != path1a {
		t.Error("Expected same cache file path for same key")
	}
}

func TestManuallyCorruptedEntry(t *testing.T) {
	tmpDir := t.TempDir()
	c, err := New(tmpDir, 24*time.Hour)
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}

	// Create a cache entry with valid JSON but wrong structure
	cacheFile := c.getCacheFilePath("test-key")
	err = os.MkdirAll(filepath.Dir(cacheFile), 0755)
	if err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Write JSON that's valid but not a proper CacheEntry
	badEntry := map[string]string{"wrong": "structure"}
	badData, _ := json.Marshal(badEntry)
	err = os.WriteFile(cacheFile, badData, 0644)
	if err != nil {
		t.Fatalf("Failed to write corrupted cache file: %v", err)
	}

	// Should handle gracefully (missing required fields)
	var data interface{}
	found, err := c.Get("test-key", &data)
	if err != nil {
		t.Fatalf("Unexpected error on corrupted entry: %v", err)
	}
	if found {
		t.Error("Expected cache miss for corrupted entry")
	}
}
