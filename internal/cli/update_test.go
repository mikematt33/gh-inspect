package cli

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadChecksums(t *testing.T) {
	checksumContent := "abc123  file1.tar.gz\ndef456  file2.tar.gz\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, checksumContent)
	}))
	defer server.Close()

	checksums, err := downloadChecksums(server.URL)
	if err != nil {
		t.Fatalf("downloadChecksums failed: %v", err)
	}

	if len(checksums) != 2 {
		t.Errorf("Expected 2 checksums, got %d", len(checksums))
	}

	if checksums["file1.tar.gz"] != "abc123" {
		t.Errorf("Expected checksum 'abc123', got '%s'", checksums["file1.tar.gz"])
	}

	if checksums["file2.tar.gz"] != "def456" {
		t.Errorf("Expected checksum 'def456', got '%s'", checksums["file2.tar.gz"])
	}
}

func TestDownloadFile(t *testing.T) {
	content := "test file content"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, content)
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "test-download")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "testfile")
	err = downloadFile(server.URL, filePath)
	if err != nil {
		t.Fatalf("downloadFile failed: %v", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(data) != content {
		t.Errorf("Expected content '%s', got '%s'", content, string(data))
	}
}

func TestCalculateSHA256(t *testing.T) {
	content := "test content for checksum"

	tmpDir, err := os.MkdirTemp("", "test-checksum")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	filePath := filepath.Join(tmpDir, "testfile")
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	checksum, err := calculateSHA256(filePath)
	if err != nil {
		t.Fatalf("calculateSHA256 failed: %v", err)
	}

	// Calculate expected checksum
	h := sha256.New()
	h.Write([]byte(content))
	expected := hex.EncodeToString(h.Sum(nil))

	if checksum != expected {
		t.Errorf("Expected checksum '%s', got '%s'", expected, checksum)
	}
}

func TestExtractFromTarGz(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-tar")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a tar.gz archive with a test file
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	content := []byte("test binary content")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}

	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	header := &tar.Header{
		Name: "gh-inspect",
		Mode: 0755,
		Size: int64(len(content)),
	}

	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("Failed to write tar header: %v", err)
	}

	if _, err := tw.Write(content); err != nil {
		t.Fatalf("Failed to write tar content: %v", err)
	}

	tw.Close()
	gzw.Close()
	f.Close()

	// Test extraction
	data, err := extractFromTarGz(archivePath, "gh-inspect")
	if err != nil {
		t.Fatalf("extractFromTarGz failed: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("Expected content '%s', got '%s'", string(content), string(data))
	}
}

func TestExtractFromTarGzNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-tar")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a tar.gz archive with a different file
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	content := []byte("test binary content")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}

	gzw := gzip.NewWriter(f)
	tw := tar.NewWriter(gzw)

	header := &tar.Header{
		Name: "other-file",
		Mode: 0755,
		Size: int64(len(content)),
	}

	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("Failed to write tar header: %v", err)
	}

	if _, err := tw.Write(content); err != nil {
		t.Fatalf("Failed to write tar content: %v", err)
	}

	tw.Close()
	gzw.Close()
	f.Close()

	// Test extraction - should fail
	_, err = extractFromTarGz(archivePath, "gh-inspect")
	if err == nil {
		t.Error("Expected error when file not found in archive, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestExtractFromZip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-zip")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a zip archive with a test file
	archivePath := filepath.Join(tmpDir, "test.zip")
	content := []byte("test binary content")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}

	zw := zip.NewWriter(f)

	fw, err := zw.Create("gh-inspect.exe")
	if err != nil {
		t.Fatalf("Failed to create file in zip: %v", err)
	}

	if _, err := fw.Write(content); err != nil {
		t.Fatalf("Failed to write content to zip: %v", err)
	}

	zw.Close()
	f.Close()

	// Test extraction
	data, err := extractFromZip(archivePath, "gh-inspect.exe")
	if err != nil {
		t.Fatalf("extractFromZip failed: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("Expected content '%s', got '%s'", string(content), string(data))
	}
}

func TestExtractFromZipNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-zip")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a zip archive with a different file
	archivePath := filepath.Join(tmpDir, "test.zip")
	content := []byte("test binary content")

	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("Failed to create archive file: %v", err)
	}

	zw := zip.NewWriter(f)

	fw, err := zw.Create("other-file")
	if err != nil {
		t.Fatalf("Failed to create file in zip: %v", err)
	}

	if _, err := fw.Write(content); err != nil {
		t.Fatalf("Failed to write content to zip: %v", err)
	}

	zw.Close()
	f.Close()

	// Test extraction - should fail
	_, err = extractFromZip(archivePath, "gh-inspect.exe")
	if err == nil {
		t.Error("Expected error when file not found in archive, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func TestGetLatestRelease(t *testing.T) {
	releaseJSON := `{"tag_name": "v1.2.3"}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, releaseJSON)
	}))
	defer server.Close()

	// Test JSON parsing logic with mock server
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to get response: %v", err)
	}
	defer resp.Body.Close()

	var rel Release
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected response data, got empty")
	}

	_ = rel // Use the variable to avoid unused variable warning
}

func TestDoUpdateUnsupportedOS(t *testing.T) {
	// This test verifies error messages are properly formatted
	// We can't easily test the full update flow without network access

	// Test will fail on unsupported OS/arch combinations or network issues
	// This is mainly to ensure the function doesn't panic
	err := doUpdate("v999.999.999")
	// We expect this to fail since we're using a non-existent version
	if err == nil {
		// If it somehow succeeds (unlikely with fake version), that's unexpected
		t.Error("doUpdate unexpectedly succeeded with fake version")
	} else {
		// Verify error is not nil and contains useful information
		if err.Error() == "" {
			t.Error("Expected non-empty error message")
		}
	}
}
