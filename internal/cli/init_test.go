package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCmd(t *testing.T) {
	// Create a temp directory for the test
	tmpDir, err := os.MkdirTemp("", "gh-inspect-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Mock XDG_CONFIG_HOME to point to tmpDir so config is written there
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", originalXDG) }()
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Change working directory to temp dir
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current wd: %v", err)
	}
	defer func() { _ = os.Chdir(originalWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change wd: %v", err)
	}

	// Run init command
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Errorf("initCmd failed: %v", err)
	}

	// Check if config.yaml was created in the expected subfolder
	// os.UserConfigDir() (mocked) + /gh-inspect/config.yaml
	configPath := filepath.Join(tmpDir, "gh-inspect", "config.yaml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("config.yaml was not created at %s", configPath)
	}

	// Verify content (simple check)
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Errorf("Failed to read config.yaml: %v", err)
	}
	if len(content) == 0 {
		t.Error("config.yaml is empty")
	}

	// Run init command again (should checking existing file and not fail/overwrite)
	rootCmd.SetArgs([]string{"init"})
	if err := rootCmd.Execute(); err != nil {
		t.Errorf("initCmd failed on second run: %v", err)
	}
}
