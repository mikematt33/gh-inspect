package cli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var completionAuto bool

// getCompletionVersion returns a hash representing the current command structure
// This is used to detect if completions are outdated
func getCompletionVersion() string {
	h := sha256.New()
	// Include version and command structure in hash
	h.Write([]byte(Version))

	// Walk through all commands to create a signature
	var walkCommands func(*cobra.Command)
	walkCommands = func(cmd *cobra.Command) {
		h.Write([]byte(cmd.Use))
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			h.Write([]byte(flag.Name))
		})
		for _, subCmd := range cmd.Commands() {
			walkCommands(subCmd)
		}
	}
	walkCommands(rootCmd)

	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `Generate and install shell completion scripts.

Automatic Configuration:
  $ gh-inspect completion --auto

  This will detect your shell (Bash/Zsh) and append the necessary setup command
  to your configuration file (.bashrc/.zshrc).

Check Completion Status:
  $ gh-inspect completion status

  Checks if installed completions match the current version.

Manual Configuration:

Bash:
  $ source <(gh-inspect completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ gh-inspect completion bash > /etc/bash_completion.d/gh-inspect
  # macOS:
  $ gh-inspect completion bash > /usr/local/etc/bash_completion.d/gh-inspect

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ gh-inspect completion zsh > "${fpath[1]}/_gh-inspect"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ gh-inspect completion fish | source

  # To load completions for each session, execute once:
  $ gh-inspect completion fish > ~/.config/fish/completions/gh-inspect.fish
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell", "status"},
	Args:                  cobra.MatchAll(cobra.ArbitraryArgs, cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		if completionAuto {
			runAutoCompletion()
			return
		}

		if len(args) == 0 {
			_ = cmd.Help()
			return
		}

		if args[0] == "status" {
			runCompletionStatus()
			return
		}

		switch args[0] {
		case "bash":
			writeCompletionHeader(os.Stdout, "bash")
			_ = cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			writeCompletionHeader(os.Stdout, "zsh")
			_ = cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			writeCompletionHeader(os.Stdout, "fish")
			_ = cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			writeCompletionHeader(os.Stdout, "powershell")
			_ = cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
	completionCmd.Flags().BoolVar(&completionAuto, "auto", false, "Automatically attempt to configure shell completion for the current shell")
}

func runAutoCompletion() {
	shell := os.Getenv("SHELL")
	if shell == "" {
		fmt.Println("❌ Could not detect shell (SHELL env var empty). Please configure manually.")
		return
	}

	var targetFile string
	var commandToAppend string
	shellName := filepath.Base(shell)

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("❌ Could not find user home directory: %v\n", err)
		return
	}

	switch shellName {
	case "bash":
		targetFile = filepath.Join(home, ".bashrc")
		commandToAppend = "source <(gh-inspect completion bash)"
	case "zsh":
		targetFile = filepath.Join(home, ".zshrc")
		commandToAppend = "source <(gh-inspect completion zsh)"
	default:
		fmt.Printf("❌ Auto-completion is currently only supported for Bash and Zsh (detected: %s).\nPlease follow the manual instructions.\n", shellName)
		return
	}

	fmt.Printf("Detected Shell: %s\n", shellName)
	fmt.Printf("Target Config File: %s\n", targetFile)
	fmt.Printf("Action: Append the following line to the file:\n  %s\n\n", commandToAppend)

	if !promptYesNo("Do you want to proceed?") {
		fmt.Println("Aborted.")
		return
	}

	// Read file content first to check for duplicates
	content, err := os.ReadFile(targetFile)
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("❌ Failed to read file: %v\n", err)
		return
	}

	existingContent := string(content)

	if strings.Contains(existingContent, "gh-inspect completion") {
		fmt.Println("⚠️  gh-inspect completion is already configured in this file.")

		// Check if it's an old version by looking for the command
		oldCommand := ""
		lines := strings.Split(existingContent, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "source") && strings.Contains(trimmed, "gh-inspect completion") {
				oldCommand = line
				break
			}
		}

		if oldCommand != "" && strings.TrimSpace(oldCommand) != strings.TrimSpace(commandToAppend) {
			fmt.Println("🔄 Detected outdated completion command. Will replace with updated version.")

			// Replace the old command with the new one
			newContent := strings.Replace(existingContent, oldCommand, commandToAppend, 1)

			if err := os.WriteFile(targetFile, []byte(newContent), 0644); err != nil {
				fmt.Printf("❌ Failed to update file: %v\n", err)
				return
			}

			fmt.Println("✅ Successfully updated completion configuration.")
			fmt.Printf("🔄 Please restart your terminal or run 'source %s' to activate.\n", targetFile)
			fmt.Println("\n💡 Run 'gh-inspect completion status' to verify your completion setup.")
			return
		}

		// Already configured and up-to-date
		fmt.Println("✅ Completion is already configured and up-to-date.")
		return
	}

	// Now open file for appending
	f, err := os.OpenFile(targetFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("❌ Failed to open file: %v\n", err)
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Printf("❌ Failed to close file: %v\n", err)
		}
	}()

	if _, err := fmt.Fprintf(f, "\n# gh-inspect completion\n%s\n", commandToAppend); err != nil {
		fmt.Printf("❌ Failed to write to file: %v\n", err)
		return
	}

	fmt.Println("✅ Successfully configured completion.")
	fmt.Printf("🔄 Please restart your terminal or run 'source %s' to activate.\n", targetFile)
	fmt.Println("\n💡 Run 'gh-inspect completion status' to verify your completion setup.")
}

// writeCompletionHeader writes version metadata as a comment in completion scripts
func writeCompletionHeader(w *os.File, shell string) {
	version := getCompletionVersion()
	var comment string
	switch shell {
	case "bash", "zsh":
		comment = "#"
	case "fish":
		comment = "#"
	case "powershell":
		comment = "#"
	}
	_, _ = fmt.Fprintf(w, "%s gh-inspect completion version: %s\n", comment, version)
	_, _ = fmt.Fprintf(w, "%s gh-inspect version: %s\n", comment, Version)
	_, _ = fmt.Fprintf(w, "%s Generated: %s\n\n", comment, "auto")
}

// runCompletionStatus checks if installed completions match current version
func runCompletionStatus() {
	shell := os.Getenv("SHELL")
	if shell == "" {
		fmt.Println("❌ Could not detect shell (SHELL env var empty)")
		return
	}

	shellName := filepath.Base(shell)
	currentVersion := getCompletionVersion()

	fmt.Printf("Current Version: %s (gh-inspect %s)\n", currentVersion, Version)
	fmt.Printf("Shell: %s\n\n", shellName)

	// Check common completion file locations based on shell
	var checkPaths []string
	home, _ := os.UserHomeDir()

	switch shellName {
	case "bash":
		checkPaths = []string{
			filepath.Join(home, ".bashrc"),
			"/etc/bash_completion.d/gh-inspect",
			"/usr/local/etc/bash_completion.d/gh-inspect",
		}
	case "zsh":
		checkPaths = []string{
			filepath.Join(home, ".zshrc"),
			// Common zsh completion paths
			"/usr/local/share/zsh/site-functions/_gh-inspect",
			"/usr/share/zsh/site-functions/_gh-inspect",
		}
	case "fish":
		checkPaths = []string{
			filepath.Join(home, ".config/fish/completions/gh-inspect.fish"),
		}
	default:
		fmt.Printf("⚠️  Completion status check not supported for %s\n", shellName)
		return
	}

	found := false
	outdated := false

	for _, path := range checkPaths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Check if file contains gh-inspect completion references
		if !strings.Contains(string(content), "gh-inspect") {
			continue
		}

		found = true
		fmt.Printf("📄 Found: %s\n", path)

		// Check version marker
		if strings.Contains(string(content), "completion version: "+currentVersion) {
			fmt.Println("   ✅ Up to date")
		} else if strings.Contains(string(content), "completion version:") {
			fmt.Println("   ⚠️  Outdated - regenerate with 'gh-inspect completion --auto'")
			outdated = true
		} else {
			fmt.Println("   ⚠️  No version marker - may need regeneration")
			outdated = true
		}
	}

	if !found {
		fmt.Println("❌ No completion configuration found")
		fmt.Println("\nRun 'gh-inspect completion --auto' to set up completions")
	} else if outdated {
		fmt.Println("\n💡 Run 'gh-inspect completion --auto' to update")
	} else {
		fmt.Println("\n✅ Completions are up to date")
	}
}
