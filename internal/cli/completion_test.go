package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionCmd(t *testing.T) {
	// Test that completion command exists and has proper metadata
	if completionCmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("completionCmd.Use = %q, want %q", completionCmd.Use, "completion [bash|zsh|fish|powershell]")
	}

	if completionCmd.Short == "" {
		t.Error("completionCmd.Short is empty")
	}

	if completionCmd.Long == "" {
		t.Error("completionCmd.Long is empty")
	}

	if completionCmd.Run == nil {
		t.Error("completionCmd.Run is nil")
	}

	// Verify valid args
	expectedArgs := []string{"bash", "zsh", "fish", "powershell", "status"}
	if len(completionCmd.ValidArgs) != len(expectedArgs) {
		t.Errorf("completionCmd.ValidArgs length = %d, want %d", len(completionCmd.ValidArgs), len(expectedArgs))
	}

	for i, arg := range expectedArgs {
		if i >= len(completionCmd.ValidArgs) || completionCmd.ValidArgs[i] != arg {
			t.Errorf("completionCmd.ValidArgs[%d] = %q, want %q", i, completionCmd.ValidArgs[i], arg)
		}
	}
}

func TestCompletionBashGeneration(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a fresh completion command to avoid state issues
	cmd := &cobra.Command{
		Use: "test-root",
	}
	cmd.AddCommand(completionCmd)

	// Run completion bash command
	cmd.SetArgs([]string{"completion", "bash"})
	err := cmd.Execute()

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("completion bash command failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Check that bash completion script was generated
	// The output should contain bash-specific completion code
	if len(output) < 100 {
		t.Errorf("Expected bash completion script, got short output: %s", output)
	}
}

func TestCompletionZshGeneration(t *testing.T) {
	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Create a fresh completion command to avoid state issues
	cmd := &cobra.Command{
		Use: "test-root",
	}
	cmd.AddCommand(completionCmd)

	// Run completion zsh command
	cmd.SetArgs([]string{"completion", "zsh"})
	err := cmd.Execute()

	// Restore stdout
	_ = w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("completion zsh command failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Check that zsh completion script was generated
	// The output should contain zsh-specific completion code
	if len(output) < 100 {
		t.Errorf("Expected zsh completion script, got short output: %s", output)
	}
}

func TestRunAutoCompletionBash(t *testing.T) {
	// Create a temp directory for the test
	tmpDir, err := os.MkdirTemp("", "gh-inspect-completion-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tmpDir)

	// Mock SHELL environment variable
	originalShell := os.Getenv("SHELL")
	defer func() { _ = os.Setenv("SHELL", originalShell) }()
	_ = os.Setenv("SHELL", "/bin/bash")

	// Create .bashrc file
	bashrcPath := filepath.Join(tmpDir, ".bashrc")
	if err := os.WriteFile(bashrcPath, []byte("# existing content\n"), 0644); err != nil {
		t.Fatalf("Failed to create .bashrc: %v", err)
	}

	// Capture stdout and mock stdin
	oldStdout := os.Stdout
	oldStdin := os.Stdin

	// Create pipes
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()

	os.Stdout = wOut
	os.Stdin = rIn

	// Write "y\n" to stdin to accept the prompt
	go func() {
		_, _ = wIn.WriteString("y\n")
		_ = wIn.Close()
	}()

	// Run auto completion
	runAutoCompletion()

	// Restore
	_ = wOut.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, rOut)
	output := buf.String()

	// Check output for expected messages
	if !strings.Contains(output, "Detected Shell: bash") {
		t.Errorf("Expected shell detection message in output, got: %s", output)
	}

	// Check if .bashrc was modified
	content, err := os.ReadFile(bashrcPath)
	if err != nil {
		t.Fatalf("Failed to read .bashrc: %v", err)
	}

	if !strings.Contains(string(content), "gh-inspect completion") {
		t.Errorf("Expected completion command in .bashrc, got: %s", string(content))
	}
}

func TestRunAutoCompletionZsh(t *testing.T) {
	// Create a temp directory for the test
	tmpDir, err := os.MkdirTemp("", "gh-inspect-completion-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tmpDir)

	// Mock SHELL environment variable
	originalShell := os.Getenv("SHELL")
	defer func() { _ = os.Setenv("SHELL", originalShell) }()
	_ = os.Setenv("SHELL", "/bin/zsh")

	// Create .zshrc file
	zshrcPath := filepath.Join(tmpDir, ".zshrc")
	if err := os.WriteFile(zshrcPath, []byte("# existing content\n"), 0644); err != nil {
		t.Fatalf("Failed to create .zshrc: %v", err)
	}

	// Capture stdout and mock stdin
	oldStdout := os.Stdout
	oldStdin := os.Stdin

	// Create pipes
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()

	os.Stdout = wOut
	os.Stdin = rIn

	// Write "y\n" to stdin to accept the prompt
	go func() {
		_, _ = wIn.WriteString("y\n")
		_ = wIn.Close()
	}()

	// Run auto completion
	runAutoCompletion()

	// Restore
	_ = wOut.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, rOut)
	output := buf.String()

	// Check output for expected messages
	if !strings.Contains(output, "Detected Shell: zsh") {
		t.Errorf("Expected shell detection message in output, got: %s", output)
	}

	// Check if .zshrc was modified
	content, err := os.ReadFile(zshrcPath)
	if err != nil {
		t.Fatalf("Failed to read .zshrc: %v", err)
	}

	if !strings.Contains(string(content), "gh-inspect completion") {
		t.Errorf("Expected completion command in .zshrc, got: %s", string(content))
	}
}

func TestRunAutoCompletionAlreadyConfigured(t *testing.T) {
	// Create a temp directory for the test
	tmpDir, err := os.MkdirTemp("", "gh-inspect-completion-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Mock home directory
	originalHome := os.Getenv("HOME")
	defer func() { _ = os.Setenv("HOME", originalHome) }()
	_ = os.Setenv("HOME", tmpDir)

	// Mock SHELL environment variable
	originalShell := os.Getenv("SHELL")
	defer func() { _ = os.Setenv("SHELL", originalShell) }()
	_ = os.Setenv("SHELL", "/bin/bash")

	// Create .bashrc file with completion already configured
	bashrcPath := filepath.Join(tmpDir, ".bashrc")
	existingContent := "# existing content\nsource <(gh-inspect completion bash)\n"
	if err := os.WriteFile(bashrcPath, []byte(existingContent), 0644); err != nil {
		t.Fatalf("Failed to create .bashrc: %v", err)
	}

	// Capture stdout and mock stdin
	oldStdout := os.Stdout
	oldStdin := os.Stdin

	// Create pipes
	rOut, wOut, _ := os.Pipe()
	rIn, wIn, _ := os.Pipe()

	os.Stdout = wOut
	os.Stdin = rIn

	// Write "y\n" twice (first for "Do you want to proceed", second for "Append anyway")
	go func() {
		_, _ = wIn.WriteString("y\ny\n")
		_ = wIn.Close()
	}()

	// Run auto completion
	runAutoCompletion()

	// Restore
	_ = wOut.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, rOut)
	output := buf.String()

	// Check output mentions already configured
	if !strings.Contains(output, "already configured") {
		t.Errorf("Expected 'already configured' message in output, got: %s", output)
	}
}

func TestRunAutoCompletionUnsupportedShell(t *testing.T) {
	// Mock SHELL environment variable to unsupported shell
	originalShell := os.Getenv("SHELL")
	defer func() { _ = os.Setenv("SHELL", originalShell) }()
	_ = os.Setenv("SHELL", "/bin/fish")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run auto completion
	runAutoCompletion()

	// Restore
	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Check output mentions unsupported
	if !strings.Contains(output, "currently only supported for Bash and Zsh") {
		t.Errorf("Expected unsupported shell message in output, got: %s", output)
	}
}
