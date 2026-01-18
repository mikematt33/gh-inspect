package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/mikematt33/gh-inspect/internal/config"
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
	Run: runAuth,
}

func init() {
	rootCmd.AddCommand(authCmd)
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

func saveToken(token string) {
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
		return
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
	// GitHub tokens are typically 20+ characters
	// Basic validation: not empty and has reasonable length
	return len(token) >= 20
}
