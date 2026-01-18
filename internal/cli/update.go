package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update gh-inspect to the latest version",
	RunE:  runUpdate,
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

	assetName := fmt.Sprintf("%s_%s_%s.tar.gz", binary, osName, archName)
	downloadUrl := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s", owner, repo, version, assetName)

	fmt.Printf("Downloading %s...\n", downloadUrl)

	// Create temp dir for download
	tmpDir, err := os.MkdirTemp("", "gh-inspect-update")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download
	resp, err := http.Get(downloadUrl)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download asset: %s", resp.Status)
	}

	// Extract
	var foundBinary io.Reader

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip reader failed: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar reading failed: %w", err)
		}

		if filepath.Base(header.Name) == binary {
			foundBinary = tr
			break
		}
	}

	if foundBinary == nil {
		return fmt.Errorf("binary '%s' not found in release archive", binary)
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
	tempDst := filepath.Join(installDir, fmt.Sprintf(".%s.new", binary))

	outf, err := os.OpenFile(tempDst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied writing to %s: please run with sudo", installDir)
		}
		return fmt.Errorf("failed to create new binary file: %w", err)
	}

	_, err = io.Copy(outf, foundBinary)
	outf.Close() // Close BEFORE renaming
	if err != nil {
		os.Remove(tempDst)
		return fmt.Errorf("failed to write new binary: %w", err)
	}

	// Atomic rename (replace)
	if err := os.Rename(tempDst, exe); err != nil {
		os.Remove(tempDst)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}
