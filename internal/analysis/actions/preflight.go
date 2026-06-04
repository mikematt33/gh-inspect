package actions

import "github.com/mikematt33/gh-inspect/pkg/models"

// QuotaSource reports remaining quota for a credential, used by the estimator.
type QuotaSource interface {
	Summaries() []models.TokenStatusSummary
	TotalAvailable() int
}

// PreflightInput describes the scope to estimate.
type PreflightInput struct {
	Repos               int
	AvgWorkflowsPerRepo float64 // estimate when not measured; 0 -> default
	SampleJobRuns       int     // per-workflow job sampling (drives job API calls)
	MaxRunsPerRepo      int
}

// EstimatePreflight projects the API cost of a scan and compares it against the
// quota currently available across all configured credentials.
//
// Cost model per repository:
//   - 1 call to list workflows
//   - 1 call per workflow to read its YAML (triggers)
//   - run pages: ceil(maxRuns / 100) calls
//   - job calls: workflows * sampleJobRuns (one ListWorkflowJobs per sampled run)
func EstimatePreflight(in PreflightInput, quota QuotaSource) models.PreflightEstimate {
	avgWF := in.AvgWorkflowsPerRepo
	if avgWF <= 0 {
		avgWF = 5
	}
	maxRuns := in.MaxRunsPerRepo
	if maxRuns <= 0 {
		maxRuns = 1000
	}

	estWorkflows := int(avgWF * float64(in.Repos))

	runPages := (maxRuns + 99) / 100
	perRepoCalls := 1          // list workflows
	perRepoCalls += int(avgWF) // YAML reads
	perRepoCalls += runPages   // run pagination
	if in.SampleJobRuns > 0 {
		perRepoCalls += int(avgWF) * in.SampleJobRuns // job calls
	}

	estCalls := perRepoCalls * in.Repos

	est := models.PreflightEstimate{
		Repos:              in.Repos,
		EstimatedWorkflows: estWorkflows,
		EstimatedAPICalls:  estCalls,
	}
	if quota != nil {
		est.Sources = quota.Summaries()
		est.AvailableQuota = quota.TotalAvailable()
	}
	est.Sufficient = est.AvailableQuota >= est.EstimatedAPICalls
	return est
}
