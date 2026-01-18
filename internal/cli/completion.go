package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var completionAuto bool

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `Generate and install shell completion scripts.

Automatic Configuration:
  $ gh-inspect completion --auto

  This will detect your shell (Bash/Zsh) and append the necessary setup command
  to your configuration file (.bashrc/.zshrc).

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
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ArbitraryArgs, cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		if completionAuto {
			runAutoCompletion()
			return
		}

		if len(args) == 0 {
			cmd.Help()
			return
		}

		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
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
		// Check for .bash_profile on Mac if .bashrc doesn't exist? 
		// Simplicity: default to .bashrc for Linux (user's OS)
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

	// Check if file exists
	f, err := os.OpenFile(targetFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("❌ Failed to open file: %v\n", err)
		return
	}
	defer f.Close()

	// Check if already present (naive check)
	content, _ := os.ReadFile(targetFile)
	if strings.Contains(string(content), "gh-inspect completion") {
		fmt.Println("⚠️  It looks like gh-inspect completion is already configured in this file.")
		if !promptYesNo("Append anyway?") {
			return
		}
	}

	if _, err := f.WriteString(fmt.Sprintf("\n# gh-inspect completion\n%s\n", commandToAppend)); err != nil {
		fmt.Printf("❌ Failed to write to file: %v\n", err)
		return
	}

	fmt.Println("✅ Successfully configured completion.")
	fmt.Printf("🔄 Please restart your terminal or run 'source %s' to activate.\n", targetFile)
}
