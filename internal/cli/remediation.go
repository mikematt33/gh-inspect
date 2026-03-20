package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mikematt33/gh-inspect/internal/config"
	"github.com/mikematt33/gh-inspect/pkg/remediation"
	"github.com/spf13/cobra"
)

var remediationNote string

var remediationCmd = &cobra.Command{
	Use:   "remediation",
	Short: "Manage finding remediation state",
}

var remediationListCmd = &cobra.Command{
	Use:   "list [repos...]",
	Short: "List findings with remediation IDs and states",
	Args:  cobra.MinimumNArgs(1),
	Run:   runRemediationList,
}

var remediationSetStatusCmd = &cobra.Command{
	Use:   "set-status [finding-id] [status]",
	Short: "Set remediation status for a finding",
	Args:  cobra.ExactArgs(2),
	Run:   runRemediationSetStatus,
}

func init() {
	rootCmd.AddCommand(remediationCmd)
	remediationCmd.AddCommand(remediationListCmd)
	remediationCmd.AddCommand(remediationSetStatusCmd)

	remediationListCmd.Flags().StringVarP(&flagSince, "since", "s", "30d", "Lookback window (e.g. 30d, 24h)")
	remediationListCmd.Flags().StringVar(&flagDepth, "depth", "standard", "Analysis depth: shallow, standard, or deep")
	remediationListCmd.Flags().IntVar(&flagMaxPRs, "max-prs", 0, "Maximum PRs to analyze (0 = use depth default)")
	remediationListCmd.Flags().IntVar(&flagMaxIssues, "max-issues", 0, "Maximum issues to fetch (0 = use depth default)")
	remediationListCmd.Flags().IntVar(&flagMaxWorkflowRuns, "max-workflow-runs", 0, "Maximum CI runs to analyze (0 = use depth default)")
	remediationListCmd.Flags().StringSliceVar(&flagInclude, "include", nil, "Only run specified analyzers")
	remediationListCmd.Flags().StringSliceVar(&flagExclude, "exclude", nil, "Exclude specified analyzers")
	remediationListCmd.Flags().StringVar(&flagRemediationFile, "remediation-file", "", "Path to remediation tracking file")

	remediationSetStatusCmd.Flags().StringVar(&flagRemediationFile, "remediation-file", "", "Path to remediation tracking file")
	remediationSetStatusCmd.Flags().StringVar(&remediationNote, "note", "", "Optional note for the remediation entry")
}

func runRemediationList(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	outputMode := resolveOutputMode(cfg)
	fullReport, err := pipelineRunner(AnalysisOptions{
		Repos:           args,
		Since:           flagSince,
		Depth:           flagDepth,
		MaxPRs:          flagMaxPRs,
		MaxIssues:       flagMaxIssues,
		MaxWorkflowRuns: flagMaxWorkflowRuns,
		Include:         flagInclude,
		Exclude:         flagExclude,
		OutputMode:      string(outputMode),
	})
	if err != nil {
		fmt.Printf("Error running analysis: %v\n", err)
		os.Exit(1)
	}

	applyRemediationTracking(fullReport)

	for _, repo := range fullReport.Repositories {
		fmt.Printf("\n%s\n", repo.Name)
		fmt.Println(strings.Repeat("=", len(repo.Name)))
		foundAny := false
		for _, analyzer := range repo.Analyzers {
			findings := make([]string, 0, len(analyzer.Findings))
			for _, finding := range analyzer.Findings {
				foundAny = true
				line := fmt.Sprintf("[%s] %s %s: %s", finding.TrackingID, finding.RemediationState, finding.Type, finding.Message)
				if finding.RemediationNote != "" {
					line += fmt.Sprintf(" (note: %s)", finding.RemediationNote)
				}
				findings = append(findings, line)
			}
			sort.Strings(findings)
			for _, line := range findings {
				fmt.Printf("  %s\n", line)
			}
		}
		if !foundAny {
			fmt.Println("  No findings found.")
		}
	}
}

func runRemediationSetStatus(cmd *cobra.Command, args []string) {
	id := args[0]
	status := args[1]
	if !remediation.IsValidStatus(status) {
		fmt.Printf("Invalid remediation status: %s\n", status)
		os.Exit(1)
	}

	path := resolveRemediationPath()
	store, err := remediation.Load(path)
	if err != nil {
		fmt.Printf("Error loading remediation store: %v\n", err)
		os.Exit(1)
	}

	remediation.SetStatus(store, id, remediation.Status(status), remediationNote)
	if err := remediation.Save(path, store); err != nil {
		fmt.Printf("Error saving remediation store: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Updated %s to %s\n", id, status)
}
