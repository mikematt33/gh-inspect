package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mikematt33/gh-inspect/internal/config"
	ghclient "github.com/mikematt33/gh-inspect/internal/github"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	flagNoBrowser bool
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with GitHub",
	Long: `Log in to GitHub to increase API rate limits and access private repositories.
This command helps you authenticate by:
1. Detecting if the GitHub CLI ('gh') is installed and using its credentials.
2. Or securely prompting for a Personal Access Token (PAT).

The token is saved to your configuration file for future use.`,
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to GitHub",
	Long:  "Authenticate with GitHub using the GitHub CLI or by providing a Personal Access Token.",
	Run:   runAuth,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check authentication status",
	Long:  "Display current authentication status and token information.",
	Run:   runAuthStatus,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out from GitHub",
	Long:  "Remove stored GitHub tokens from configuration file and shell rc files (.bashrc, .zshrc, etc.).",
	Run:   runAuthLogout,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)

	// Add flags
	authCmd.PersistentFlags().BoolVar(&flagNoBrowser, "no-browser", false, "Disable browser-based authentication (use device code flow)")
	authLoginCmd.Flags().BoolVar(&flagNoBrowser, "no-browser", false, "Disable browser-based authentication (use device code flow)")
}

func runAuth(cmd *cobra.Command, args []string) {
	fmt.Println("GitHub Authentication Status")
	fmt.Println("----------------------------")

	// Check for existing authentication
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("⚠️  Error loading config: %v\n", err)
		cfg = nil
	}

	var configToken string
	if cfg != nil {
		configToken = cfg.Global.GitHubToken
	}

	token := ghclient.ResolveToken(configToken)
	if token != "" {
		fmt.Println("✅ You are already authenticated!")
		fmt.Println()

		// Show where the token is from
		if configToken != "" && configToken == token {
			fmt.Println("Token source: Config file")
		} else if checkGhCLIToken() {
			fmt.Println("Token source: GitHub CLI (gh)")
		} else {
			fmt.Println("Token source: GITHUB_TOKEN environment variable")
		}

		// Validate the token
		if err := validateToken(token); err != nil {
			fmt.Printf("⚠️  Current token is invalid: %v\n", err)
			fmt.Println()
		} else {
			fmt.Println("Token status: Valid")
			fmt.Println()
		}
		fmt.Println()

		if !promptYesNo("Do you want to change your authentication?") {
			fmt.Println("No changes made.")
			return
		}
		fmt.Println()
	}

	// Start new authentication flow
	fmt.Println("Authenticate with GitHub")
	fmt.Println("------------------------")
	fmt.Println()

	// Check if 'gh' is available
	ghPath, err := exec.LookPath("gh")
	if err == nil {
		fmt.Printf("Detected GitHub CLI (gh) at %s\n", ghPath)
		if promptYesNo("Do you want to login using the GitHub CLI? (Recommended)") {
			loginWithGh()
			return
		}
		fmt.Println()
	} else {
		fmt.Println()
		fmt.Println("GitHub CLI (gh) not found.")
	}

	loginWithToken()
}

func checkGhCLIToken() bool {
	cmd := exec.Command("gh", "auth", "token")
	return cmd.Run() == nil
}

func loginWithGh() {
	// Check if already logged in via gh
	cmd := exec.Command("gh", "auth", "token")
	if err := cmd.Run(); err == nil {
		fmt.Println("✅ You are already logged in via GitHub CLI.")
		tokenBytes, err := exec.Command("gh", "auth", "token").Output()
		if err != nil {
			fmt.Printf("❌ Failed to retrieve token: %v\n", err)
			return
		}
		token := strings.TrimSpace(string(tokenBytes))
		if !isValidToken(token) {
			fmt.Println("❌ Retrieved token is invalid or empty.")
			return
		}
		saveToken(token)
		return
	}

	// Run login
	var loginArgs []string
	if flagNoBrowser {
		fmt.Println("Running 'gh auth login --web' (device code flow)...")
		loginArgs = []string{"auth", "login", "--web"}
	} else {
		fmt.Println("Running 'gh auth login'...")
		loginArgs = []string{"auth", "login"}
	}
	cmd = exec.Command("gh", loginArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Login failed: %v\n", err)
		if promptYesNo("Try pasting a token manually instead?") {
			loginWithToken()
		}
		return
	}

	// Fetch token after login
	tokenBytes, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		fmt.Println("❌ Failed to retrieve token after login.")
		return
	}

	token := strings.TrimSpace(string(tokenBytes))
	if !isValidToken(token) {
		fmt.Println("❌ Retrieved token is invalid or empty.")
		return
	}

	saveToken(token)
}

func loginWithToken() {
	fmt.Println("\nPlease generate a Personal Access Token (PAT) with 'repo' scope.")
	fmt.Println("Generate one here: https://github.com/settings/tokens/new?scopes=repo&description=gh-inspect")
	fmt.Print("\nPaste your token: ")

	byteToken, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		fmt.Println("\n❌ Failed to read token.")
		// Fallback to simple read if term fails (e.g. windows mintty sometimes)
		reader := bufio.NewReader(os.Stdin)
		tokenStr, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("❌ Failed to read token from standard input.")
			return
		}
		tokenStr = strings.TrimSpace(tokenStr)
		if tokenStr == "" {
			fmt.Println("❌ Empty token provided.")
			return
		}
		saveToken(tokenStr)
		return
	}
	token := strings.TrimSpace(string(byteToken))
	fmt.Println() // Newline after input

	if token == "" {
		fmt.Println("❌ Empty token provided.")
		return
	}

	saveToken(token)
}

// validateToken checks if a token is valid by making an API call
// This is a variable to allow mocking in tests
var validateToken = func(token string) error {
	client := ghclient.NewClient(token)
	_, err := client.GetRateLimit(context.Background())
	return err
}

func saveToken(token string) {
	// Validate token with GitHub API before saving
	fmt.Println("Validating token...")
	err := validateToken(token)
	if err != nil {
		fmt.Printf("❌ Token validation failed: %v\n", err)
		fmt.Println("The token may be invalid or expired. Please check and try again.")
		return
	}

	fmt.Println()
	fmt.Println("✅ Token validated successfully!")

	// Ask user where to store the token
	chooseTokenStorage(token)
}

func chooseTokenStorage(token string) {
	fmt.Println("How would you like to store your GitHub token?")
	fmt.Println()
	fmt.Println("1. Temporary (export for current session only)")
	fmt.Println("2. Persistent shell (add to .bashrc/.zshrc)")
	fmt.Println("3. Config file (store in gh-inspect config)")
	fmt.Println("4. Don't store (I'll use gh CLI or GITHUB_TOKEN)")
	fmt.Println()
	fmt.Print("Enter choice [1-4]: ")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		storeTokenTemporary(token)
	case "2":
		storeTokenPersistentShell(token)
	case "3":
		storeTokenConfig(token)
	case "4":
		fmt.Println("\n✅ Token validated but not stored.")
		fmt.Println("💡 Use 'export GITHUB_TOKEN=\"your_token\"' or 'gh auth login' to authenticate.")
	default:
		fmt.Println("\n❌ Invalid choice. Token not stored.")
	}
}

func storeTokenTemporary(token string) {
	fmt.Println("\n✅ To use this token temporarily, run:")
	fmt.Println()
	fmt.Printf("  export GITHUB_TOKEN=\"%s\"\n", token)
	fmt.Println()
	fmt.Println("This will only be available in your current terminal session.")
}

func storeTokenPersistentShell(token string) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		fmt.Println("\n❌ Could not detect shell. Please add manually.")
		return
	}

	shellName := filepath.Base(shell)
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("\n❌ Could not find home directory: %v\n", err)
		return
	}

	var targetFile string
	switch shellName {
	case "bash":
		targetFile = filepath.Join(home, ".bashrc")
	case "zsh":
		targetFile = filepath.Join(home, ".zshrc")
	default:
		fmt.Printf("\n⚠️  Shell '%s' not directly supported. Add this line to your shell config:\n", shellName)
		fmt.Printf("  export GITHUB_TOKEN=\"%s\"\n", token)
		return
	}

	fmt.Printf("\nThis will add 'export GITHUB_TOKEN=...' to %s\n", targetFile)
	fmt.Println("⚠️  WARNING: This stores the token in plain text in your shell config.")

	if !promptYesNo("Continue?") {
		fmt.Println("Aborted.")
		return
	}

	// Read existing content to check for duplicates
	content, _ := os.ReadFile(targetFile)
	existingContent := string(content)

	if strings.Contains(existingContent, "GITHUB_TOKEN=") {
		fmt.Println("\n⚠️  Found existing GITHUB_TOKEN entries in file.")

		// Remove only the gh-inspect-managed block and optionally other token lines with user confirmation
		lines := strings.Split(existingContent, "\n")
		var newLines []string

		for i := 0; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)

			// Automatically remove the block previously added by gh-inspect:
			// the marker comment and the following GITHUB_TOKEN export line
			if trimmed == "# GitHub token for gh-inspect" {
				if i+1 < len(lines) && strings.Contains(lines[i+1], "GITHUB_TOKEN=") {
					i++ // skip the following export line as well
				}
				continue
			}

			if strings.Contains(line, "GITHUB_TOKEN=") {
				// Preserve commented-out lines
				if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "//") {
					newLines = append(newLines, line)
					continue
				}

				fmt.Printf("\nFound existing GITHUB_TOKEN line:\n  %s\n", strings.TrimSpace(line))
				if promptYesNo("Do you want to remove this line and replace it with a new token for gh-inspect?") {
					// Skip this line (do not append), effectively removing it
					continue
				}
				// User chose to keep this line
			}

			newLines = append(newLines, line)
		}
		existingContent = strings.Join(newLines, "\n")
	}

	// Prepare new content with appended token
	// Ensure proper spacing: add newline separator only if existing content doesn't end with one
	separator := "\n"
	if existingContent != "" && !strings.HasSuffix(existingContent, "\n") {
		separator = "\n"
	} else if existingContent == "" {
		separator = ""
	}
	newContent := fmt.Sprintf("%s%s# GitHub token for gh-inspect\nexport GITHUB_TOKEN=\"%s\"\n", existingContent, separator, token)

	// Write to a temporary file first to avoid data loss on write failure
	dir := filepath.Dir(targetFile)
	tmpFile, err := os.CreateTemp(dir, ".gh-inspect-token-*")
	if err != nil {
		fmt.Printf("\n❌ Failed to create temporary file: %v\n", err)
		return
	}
	tmpName := tmpFile.Name()

	if _, err := tmpFile.WriteString(newContent); err != nil {
		fmt.Printf("\n❌ Failed to write to temporary file: %v\n", err)
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return
	}

	if err := tmpFile.Close(); err != nil {
		fmt.Printf("\n❌ Failed to close temporary file: %v\n", err)
		_ = os.Remove(tmpName)
		return
	}

	// Ensure file has the desired permissions
	if err := os.Chmod(tmpName, 0644); err != nil {
		fmt.Printf("\n❌ Failed to set permissions on temporary file: %v\n", err)
		_ = os.Remove(tmpName)
		return
	}

	// Atomically replace the target file with the new content
	if err := os.Rename(tmpName, targetFile); err != nil {
		fmt.Printf("\n❌ Failed to replace shell configuration file: %v\n", err)
		_ = os.Remove(tmpName)
		return
	}

	fmt.Println("\n✅ Token added to shell configuration.")
	fmt.Printf("🔄 Restart your terminal or run 'source %s' to activate.\n", targetFile)
}

func storeTokenConfig(token string) {
	fmt.Println("\n⚠️  WARNING: Storing token in config file as plain text.")
	fmt.Println("Consider using 'gh auth login' or environment variables for better security.")

	if !promptYesNo("\nContinue with config file storage?") {
		fmt.Println("Aborted.")
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("\n❌ Error loading config: %v\n", err)
		return
	}

	if cfg == nil {
		fmt.Println("\n❌ Error: Config structure nil")
		return
	}

	cfg.Global.GitHubToken = token
	if err := saveConfig(cfg); err != nil {
		fmt.Printf("\n❌ Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n✅ Token saved to configuration file.")
}

func promptYesNo(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [Y/n]: ", question)
	text, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	text = strings.TrimSpace(strings.ToLower(text))
	return text == "" || text == "y" || text == "yes"
}

// isValidToken checks if a token string is valid (non-empty and has expected format).
func isValidToken(token string) bool {
	// GitHub tokens vary in format and length (e.g., classic PATs are 40 chars with ghp_ prefix,
	// fine-grained PATs start with github_pat_). This performs basic validation for minimum length.
	return len(token) >= 20
}

func runAuthStatus(cmd *cobra.Command, args []string) {
	fmt.Println("GitHub Authentication Status")
	fmt.Println("----------------------------")

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("❌ Error loading config: %v\n", err)
		os.Exit(1)
	}

	token := ghclient.ResolveToken(cfg.Global.GitHubToken)
	if token == "" {
		fmt.Println("❌ Not authenticated")
		fmt.Println("\nRun 'gh-inspect auth' to log in.")
		os.Exit(1)
	}

	// Validate token and get info
	err = validateToken(token)
	if err != nil {
		fmt.Println("❌ Token is invalid or expired")
		fmt.Printf("   Error: %v\n", err)
		fmt.Println("\nRun 'gh-inspect auth' to log in again.")
		os.Exit(1)
	}

	// Get rate limit info
	client := ghclient.NewClient(token)
	limits, err := client.GetRateLimit(context.Background())
	if err != nil {
		fmt.Println("✅ Authenticated (token is valid)")
		fmt.Printf("   Could not fetch rate limit info: %v\n", err)
		return
	}

	fmt.Println("✅ Authenticated")
	fmt.Printf("   Rate limit: %d/%d remaining\n", limits.Remaining, limits.Limit)
	if !limits.Reset.IsZero() {
		// Calculate time until reset
		timeUntilReset := time.Until(limits.Reset.Time)
		var humanReadable string
		if timeUntilReset < time.Minute {
			humanReadable = fmt.Sprintf("in %d seconds", int(timeUntilReset.Seconds()))
		} else if timeUntilReset < time.Hour {
			humanReadable = fmt.Sprintf("in %d minutes", int(timeUntilReset.Minutes()))
		} else {
			humanReadable = fmt.Sprintf("in %.1f hours", timeUntilReset.Hours())
		}
		fmt.Printf("   Resets at: %s (%s)\n", limits.Reset.Format(time.RFC3339), humanReadable)
	}

	// Show token source
	if cfg.Global.GitHubToken != "" {
		fmt.Println("   Token source: config file")
	} else {
		fmt.Println("   Token source: environment or gh CLI")
	}
}

func runAuthLogout(cmd *cobra.Command, args []string) {
	fmt.Println("GitHub Authentication Logout")
	fmt.Println("---------------------------")
	fmt.Println()

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("❌ Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Check all possible token locations
	var foundLocations []string
	hasConfigToken := cfg.Global.GitHubToken != ""

	if hasConfigToken {
		foundLocations = append(foundLocations, "config file")
	}

	// Check for GITHUB_TOKEN in environment
	if os.Getenv("GITHUB_TOKEN") != "" {
		foundLocations = append(foundLocations, "GITHUB_TOKEN environment variable")
	}

	// Check for token in shell rc files
	homeDir, _ := os.UserHomeDir()
	shellFiles := []string{".bashrc", ".zshrc", ".bash_profile", ".profile"}
	var foundShellFiles []string
	for _, shellFile := range shellFiles {
		targetFile := filepath.Join(homeDir, shellFile)
		content, err := os.ReadFile(targetFile)
		if err == nil && strings.Contains(string(content), "GITHUB_TOKEN=") {
			foundShellFiles = append(foundShellFiles, shellFile)
			foundLocations = append(foundLocations, shellFile)
		}
	}

	// Check gh CLI
	if checkGhCLIToken() {
		foundLocations = append(foundLocations, "gh CLI")
	}

	if len(foundLocations) == 0 {
		fmt.Println("❌ No stored tokens found.")
		return
	}

	fmt.Println("Found tokens in the following locations:")
	for i, loc := range foundLocations {
		fmt.Printf("  %d. %s\n", i+1, loc)
	}
	fmt.Println()

	// Confirm logout
	if !promptYesNo("Do you want to remove these tokens?") {
		fmt.Println("Logout cancelled.")
		return
	}
	fmt.Println()

	// Remove from config
	if hasConfigToken {
		cfg.Global.GitHubToken = ""
		if err := saveConfig(cfg); err != nil {
			fmt.Printf("❌ Failed to save config: %v\n", err)
		} else {
			fmt.Println("✅ Removed token from config file")
		}
	}

	// Remove from shell rc files
	for _, shellFile := range foundShellFiles {
		targetFile := filepath.Join(homeDir, shellFile)
		content, err := os.ReadFile(targetFile)
		if err != nil {
			fmt.Printf("❌ Failed to read %s: %v\n", shellFile, err)
			continue
		}

		// Remove GITHUB_TOKEN lines that were added by gh-inspect
		lines := strings.Split(string(content), "\n")
		var newLines []string
		for i := 0; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)

			// If this is our marker comment, skip it and the following export line
			if trimmed == "# GitHub token for gh-inspect" {
				// Skip the marker comment itself
				// Also skip the next line if it looks like a GITHUB_TOKEN definition
				if i+1 < len(lines) {
					nextTrimmed := strings.TrimSpace(lines[i+1])
					if strings.HasPrefix(nextTrimmed, "export GITHUB_TOKEN=") || strings.HasPrefix(nextTrimmed, "GITHUB_TOKEN=") {
						i++
					}
				}
				continue
			}

			// Keep all other lines, including standalone GITHUB_TOKEN exports that weren't added by gh-inspect
			newLines = append(newLines, line)
		}

		err = os.WriteFile(targetFile, []byte(strings.Join(newLines, "\n")), 0644)
		if err != nil {
			fmt.Printf("❌ Failed to update %s: %v\n", shellFile, err)
		} else {
			fmt.Printf("✅ Removed token from %s\n", shellFile)
		}
	}

	// Provide instructions for other locations
	if os.Getenv("GITHUB_TOKEN") != "" {
		fmt.Println()
		fmt.Println("⚠️  GITHUB_TOKEN environment variable is set in your current session.")
		fmt.Println("   To remove it, run: unset GITHUB_TOKEN")
		fmt.Println("   (This will only affect the current terminal session)")
	}

	if checkGhCLIToken() {
		fmt.Println()
		fmt.Println("⚠️  GitHub CLI (gh) is authenticated.")
		fmt.Println("   To log out, run: gh auth logout")
	}

	fmt.Println()
	fmt.Println("✅ Logout complete.")
}
