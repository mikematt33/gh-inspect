package analysis

// DepthConfig defines limits for API pagination and data fetching
type DepthConfig struct {
	Name            string
	MaxPRs          int
	MaxIssues       int
	MaxWorkflowRuns int
	IncludeDeep     bool // For backward compatibility with Config.IncludeDeep
}

// Predefined depth configurations
var (
	ShallowDepth = DepthConfig{
		Name:            "shallow",
		MaxPRs:          50,
		MaxIssues:       100,
		MaxWorkflowRuns: 50,
		IncludeDeep:     false,
	}

	StandardDepth = DepthConfig{
		Name:            "standard",
		MaxPRs:          100,
		MaxIssues:       200,
		MaxWorkflowRuns: 100,
		IncludeDeep:     false,
	}

	DeepDepth = DepthConfig{
		Name:            "deep",
		MaxPRs:          500,
		MaxIssues:       1000,
		MaxWorkflowRuns: 500,
		IncludeDeep:     true,
	}
)

// GetDepthConfig returns the appropriate depth configuration
func GetDepthConfig(depth string) DepthConfig {
	switch depth {
	case "shallow":
		return ShallowDepth
	case "standard":
		return StandardDepth
	case "deep":
		return DeepDepth
	default:
		return StandardDepth
	}
}

// ApplyOverrides applies manual overrides to a depth configuration
func (d DepthConfig) ApplyOverrides(maxPRs, maxIssues, maxWorkflowRuns int) DepthConfig {
	if maxPRs > 0 {
		d.MaxPRs = maxPRs
	}
	if maxIssues > 0 {
		d.MaxIssues = maxIssues
	}
	if maxWorkflowRuns > 0 {
		d.MaxWorkflowRuns = maxWorkflowRuns
	}
	return d
}
