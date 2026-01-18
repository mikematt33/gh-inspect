package cli

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update gh-inspect to the latest version",
	Long: `Download and install the latest release of gh-inspect from GitHub.
This command replaces the current binary with the latest version available.`,
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Println("Checking for updates...")
	latest, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to get latest release: %w", err)
	}

	if latest.TagName == Version {
		fmt.Printf("You are already using the latest version: %s\n", Version)
		return nil
	}

	fmt.Printf("Updating from %s to %s...\n", Version, latest.TagName)

	if err := doUpdate(latest.TagName); err != nil {
		return err
	}

	fmt.Printf("Successfully updated to %s\n", latest.TagName)
	fmt.Println("\nNote: Please restart your terminal or re-run the command to use the new version.")
	return nil
}

type Release struct {
	TagName string `json:"tag_name"`
}

func getLatestRelease() (*Release, error) {
	resp, err := http.Get("https://api.github.com/repos/mikematt33/gh-inspect/releases/latest")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %v", resp.Status)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func doUpdate(version string) error {
	owner := "mikematt33"
	repo := "gh-inspect"
	binary := "gh-inspect"

	osType := runtime.GOOS
	arch := runtime.GOARCH

	// Map to goreleaser/release naming convention
	// install.sh logic: gh-inspect_Linux_x86_64.tar.gz
	var osName string
	switch osType {
	case "linux":
		osName = "Linux"
	case "darwin":
		osName = "Darwin"
	case "windows":
		osName = "Windows"
	default:
		return fmt.Errorf("unsupported OS: %s", osType)
	}

	var archName string
	switch arch {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "arm64"
	case "386":
		archName = "i386"
	default:
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Determine archive format based on OS
	assetExt := "tar.gz"
	if osName == "Windows" {
		assetExt = "zip"
	}

	assetName := fmt.Sprintf("%s_%s_%s.%s", binary, osName, archName, assetExt)
	checksumFile := "checksums.txt"
	downloadUrl := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, version, assetName)
	checksumUrl := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, version, checksumFile)

	fmt.Printf("Downloading %s...\n", downloadUrl)

	// Create temp dir for download
	tmpDir, err := os.MkdirTemp("", "gh-inspect-update")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download checksums file
	fmt.Println("Downloading checksums...")
	checksums, err := downloadChecksums(checksumUrl)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}

	expectedChecksum, ok := checksums[assetName]
	if !ok {
		return fmt.Errorf("checksum not found for %s", assetName)
	}

	// Download archive to temp file
	archivePath := filepath.Join(tmpDir, assetName)
	if err := downloadFile(downloadUrl, archivePath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify checksum
	fmt.Println("Verifying checksum...")
	actualChecksum, err := calculateSHA256(archivePath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	// Extract binary from archive
	var binaryData []byte
	if osName == "Windows" {
		binaryData, err = extractFromZip(archivePath, binary+".exe")
	} else {
		binaryData, err = extractFromTarGz(archivePath, binary)
	}
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	// Locate current binary
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate executable: %w", err)
	}
	
	realPath, err := filepath.EvalSymlinks(exe)
	if err == nil {
		exe = realPath
	}

	// Write new binary to a temp file first (in the same directory to allow atomic move)
	installDir := filepath.Dir(exe)
	tempDst := filepath.Join(installDir, fmt.Sprintf(".%s.new", filepath.Base(exe)))

	err = os.WriteFile(tempDst, binaryData, 0755)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied writing to %s: please run with sudo", installDir)
		}
		return fmt.Errorf("failed to create new binary file: %w", err)
	}

	// Atomic rename (replace)
	if err := os.Rename(tempDst, exe); err != nil {
		os.Remove(tempDst)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

// downloadChecksums downloads and parses the checksums.txt file
func downloadChecksums(url string) (map[string]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download checksums: %s", resp.Status)
	}

	checksums := make(map[string]string)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			checksums[parts[1]] = parts[0]
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return checksums, nil
}

// downloadFile downloads a file from url to filepath
func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// calculateSHA256 calculates the SHA256 checksum of a file
func calculateSHA256(filepath string) (string, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractFromTarGz extracts a specific file from a tar.gz archive and returns its content
func extractFromTarGz(archivePath, targetFile string) ([]byte, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if filepath.Base(header.Name) == targetFile {
			// Read the entire binary into memory
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("file '%s' not found in archive", targetFile)
}

// extractFromZip extracts a specific file from a zip archive and returns its content
func extractFromZip(archivePath, targetFile string) ([]byte, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == targetFile {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("file '%s' not found in archive", targetFile)
}
