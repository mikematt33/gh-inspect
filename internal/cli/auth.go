package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/mikematt33/gh-inspect/internal/config"
	ghclient "github.com/mikematt33/gh-inspect/internal/github"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	Long:  "Remove the stored GitHub token from your configuration.",
	Run:   runAuthLogout,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)

	// Make login the default subcommand behavior when no subcommand is specified
	authCmd.Run = runAuth
}

func runAuth(cmd *cobra.Command, args []string) {
	fmt.Println("Authenticate with GitHub")
	fmt.Println("------------------------")

	// Check if 'gh' is available
	ghPath, err := exec.LookPath("gh")
	if err == nil {
		fmt.Printf("Detected GitHub CLI (gh) at %s\n", ghPath)
		if promptYesNo("Do you want to login using the GitHub CLI? (Recommended)") {
			loginWithGh()
			return
		}
	} else {
		fmt.Println("GitHub CLI (gh) not found.")
	}

	loginWithToken()
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
	fmt.Println("Running 'gh auth login'...")
	cmd = exec.Command("gh", "auth", "login")
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

	fmt.Println("✅ Token validated successfully!")

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	if cfg == nil {
		// Should not happen with Load(), but safety check
		fmt.Println("Error: Config structure nil")
		return
	}

	cfg.Global.GitHubToken = token
	if err := saveConfig(cfg); err != nil {
		fmt.Printf("❌ Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Token saved successfully to configuration.")
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
		fmt.Printf("   Resets at: %s\n", limits.Reset.Format(time.RFC3339))
	}

	// Show token source
	if cfg.Global.GitHubToken != "" {
		fmt.Println("   Token source: config file")
	} else {
		fmt.Println("   Token source: environment or gh CLI")
	}
}

func runAuthLogout(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("❌ Error loading config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Global.GitHubToken == "" {
		fmt.Println("No token stored in configuration file.")
		fmt.Println("\nNote: If you're using GITHUB_TOKEN environment variable or gh CLI,")
		fmt.Println("you'll need to clear those separately.")
		return
	}

	// Confirm logout
	if !promptYesNo("Are you sure you want to remove the stored token?") {
		fmt.Println("Logout cancelled.")
		return
	}

	cfg.Global.GitHubToken = ""
	if err := saveConfig(cfg); err != nil {
		fmt.Printf("❌ Failed to save config: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Successfully logged out. Token removed from configuration.")
}
