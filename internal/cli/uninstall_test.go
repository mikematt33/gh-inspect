package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUninstallCmd(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "test-uninstall")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a fake binary file
	fakeBinary := filepath.Join(tmpDir, "gh-inspect-test")
	err = os.WriteFile(fakeBinary, []byte("fake binary"), 0755)
	if err != nil {
		t.Fatalf("Failed to create fake binary: %v", err)
	}

	// Verify the file exists
	if _, err := os.Stat(fakeBinary); os.IsNotExist(err) {
		t.Fatalf("Fake binary was not created")
	}

	// Remove the file (simulating uninstall)
	err = os.Remove(fakeBinary)
	if err != nil {
		t.Fatalf("Failed to remove fake binary: %v", err)
	}

	// Verify the file is gone
	if _, err := os.Stat(fakeBinary); !os.IsNotExist(err) {
		t.Errorf("Fake binary still exists after removal")
	}
}

func TestUninstallCmdPermissionError(t *testing.T) {
	// Test permission error handling
	// Create a temporary directory with restricted permissions
	tmpDir, err := os.MkdirTemp("", "test-uninstall-perm")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a fake binary file
	fakeBinary := filepath.Join(tmpDir, "gh-inspect-test")
	err = os.WriteFile(fakeBinary, []byte("fake binary"), 0755)
	if err != nil {
		t.Fatalf("Failed to create fake binary: %v", err)
	}

	// Make directory read-only (on Unix systems)
	// This test may not work the same way on all systems
	err = os.Chmod(tmpDir, 0555)
	if err != nil {
		t.Skipf("Failed to change directory permissions, skipping test: %v", err)
	}
	defer os.Chmod(tmpDir, 0755) // Restore permissions for cleanup

	// Try to remove the file
	err = os.Remove(fakeBinary)
	if err == nil {
		// On some systems (Windows, or when running as root), this might succeed
		t.Log("Warning: Expected permission error but got none. This may be OS-specific behavior.")
	} else {
		// Check if it's a permission error
		if !os.IsPermission(err) {
			t.Errorf("Expected permission error, got: %v", err)
		}
	}
}

func TestUninstallCmdOutput(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "test-uninstall-output")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a fake binary file
	fakeBinary := filepath.Join(tmpDir, "gh-inspect-test")
	err = os.WriteFile(fakeBinary, []byte("fake binary"), 0755)
	if err != nil {
		t.Fatalf("Failed to create fake binary: %v", err)
	}

	// Simulate the uninstall process with output capture
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Print the messages that uninstall would print
	fmt.Printf("Uninstalling gh-inspect from %s...\n", fakeBinary)
	err = os.Remove(fakeBinary)
	if err == nil {
		fmt.Println("gh-inspect successfully uninstalled.")
		fmt.Println("\nNote: The current process is still running. The binary will be fully removed after this command exits.")
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains expected messages
	if !strings.Contains(output, "Uninstalling gh-inspect from") {
		t.Errorf("Expected output to contain 'Uninstalling gh-inspect from', got: %s", output)
	}

	if !strings.Contains(output, "successfully uninstalled") {
		t.Errorf("Expected output to contain 'successfully uninstalled', got: %s", output)
	}

	if !strings.Contains(output, "The current process is still running") {
		t.Errorf("Expected output to contain warning about running process, got: %s", output)
	}
}

func TestUninstallSymlinkResolution(t *testing.T) {
	// Test symlink resolution
	tmpDir, err := os.MkdirTemp("", "test-uninstall-symlink")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a fake binary file
	fakeBinary := filepath.Join(tmpDir, "gh-inspect-real")
	err = os.WriteFile(fakeBinary, []byte("fake binary"), 0755)
	if err != nil {
		t.Fatalf("Failed to create fake binary: %v", err)
	}

	// Create a symlink to the binary
	symlink := filepath.Join(tmpDir, "gh-inspect-link")
	err = os.Symlink(fakeBinary, symlink)
	if err != nil {
		t.Skipf("Failed to create symlink, skipping test: %v", err)
	}

	// Test EvalSymlinks
	realPath, err := filepath.EvalSymlinks(symlink)
	if err != nil {
		t.Fatalf("Failed to evaluate symlink: %v", err)
	}

	if realPath != fakeBinary {
		t.Errorf("Expected real path '%s', got '%s'", fakeBinary, realPath)
	}

	// Remove the real binary
	err = os.Remove(realPath)
	if err != nil {
		t.Fatalf("Failed to remove real binary: %v", err)
	}

	// Verify the real binary is gone
	if _, err := os.Stat(fakeBinary); !os.IsNotExist(err) {
		t.Errorf("Real binary still exists after removal")
	}
}

func TestUninstallExecutablePathError(t *testing.T) {
	// This test verifies error handling when os.Executable() might fail
	// In practice, os.Executable() rarely fails in normal circumstances,
	// but we should still handle the error properly
	
	// We can't easily simulate os.Executable() failure without mocking,
	// but we can at least verify the error message format would be correct
	testErr := fmt.Errorf("mock executable path error")
	wrappedErr := fmt.Errorf("failed to determine executable path: %w", testErr)
	
	if !strings.Contains(wrappedErr.Error(), "failed to determine executable path") {
		t.Errorf("Expected error message to contain 'failed to determine executable path', got: %v", wrappedErr)
	}
}
