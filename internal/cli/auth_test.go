package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsValidToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "valid token",
			token:    "ghp_1234567890abcdefghijklmnopqrstuvwxyz",
			expected: true,
		},
		{
			name:     "short token",
			token:    "short",
			expected: false,
		},
		{
			name:     "empty token",
			token:    "",
			expected: false,
		},
		{
			name:     "exactly 20 chars",
			token:    "12345678901234567890",
			expected: true,
		},
		{
			name:     "19 chars",
			token:    "1234567890123456789",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidToken(tt.token)
			if result != tt.expected {
				t.Errorf("isValidToken(%q) = %v, want %v", tt.token, result, tt.expected)
			}
		})
	}
}

func TestPromptYesNo(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "yes response",
			input:    "yes\n",
			expected: true,
		},
		{
			name:     "y response",
			input:    "y\n",
			expected: true,
		},
		{
			name:     "Y response uppercase",
			input:    "Y\n",
			expected: true,
		},
		{
			name:     "empty response (default yes)",
			input:    "\n",
			expected: true,
		},
		{
			name:     "no response",
			input:    "no\n",
			expected: false,
		},
		{
			name:     "n response",
			input:    "n\n",
			expected: false,
		},
		{
			name:     "random text",
			input:    "random\n",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a pipe to simulate stdin
			oldStdin := os.Stdin
			r, w, _ := os.Pipe()
			os.Stdin = r
			defer func() {
				os.Stdin = oldStdin
			}()

			// Write test input
			go func() {
				_, _ = w.WriteString(tt.input)
				_ = w.Close()
			}()
			result := promptYesNo("Test question")
			if result != tt.expected {
				t.Errorf("promptYesNo() with input %q = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func SkipTestSaveToken(t *testing.T) {
	// Save original validator
	originalValidateToken := validateToken
	defer func() { validateToken = originalValidateToken }()

	// Mock validation to always succeed
	validateToken = func(token string) error {
		return nil
	}

	// Create a temp directory for the test
	tmpDir, err := os.MkdirTemp("", "gh-inspect-auth-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Mock XDG_CONFIG_HOME to point to tmpDir
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", originalXDG) }()
	_ = os.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a default config first
	configPath := filepath.Join(tmpDir, "gh-inspect", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write minimal config
	defaultConfig := `global:
  concurrency: 5
  github_token: ""
`
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		t.Fatalf("Failed to write default config: %v", err)
	}

	// Test saveToken
	testToken := "test_token_1234567890"
	saveToken(testToken)

	// Restore stdout and capture output
	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Check output contains success message
	if !strings.Contains(output, "✅ Token saved successfully") {
		t.Errorf("Expected success message in output, got: %s", output)
	}

	// Verify token was saved in config
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if !strings.Contains(string(content), testToken) {
		t.Errorf("Token not found in config file. Content: %s", string(content))
	}
}

func TestAuthCmd(t *testing.T) {
	// Test that auth command exists and has proper metadata
	if authCmd.Use != "auth" {
		t.Errorf("authCmd.Use = %q, want %q", authCmd.Use, "auth")
	}

	if authCmd.Short == "" {
		t.Error("authCmd.Short is empty")
	}

	if authCmd.Long == "" {
		t.Error("authCmd.Long is empty")
	}

	// Auth command no longer has a Run function - it shows help by default
	// The actual auth logic is in subcommands: login, status, logout
	if len(authCmd.Commands()) == 0 {
		t.Error("authCmd has no subcommands")
	}
}
