package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the gh-inspect CLI",
	Long: `Removes the gh-inspect binary from the system.
This command attempts to locate the binary and remove it. It does not remove configuration files.`,
	Example: "  gh-inspect uninstall",
	RunE:    runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to determine executable path: %w", err)
	}

	// Resolve symlinks to find the real binary
	realPath, err := filepath.EvalSymlinks(exe)
	if err != nil {
		// If evaluating symlinks fails, fallback to the executable path
		realPath = exe
	}

	fmt.Printf("Uninstalling gh-inspect from %s...\n", realPath)

	err = os.Remove(realPath)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: you may need to run this command with sudo")
		}
		return fmt.Errorf("failed to remove binary: %w", err)
	}

	fmt.Println("gh-inspect successfully uninstalled.")
	return nil
}
