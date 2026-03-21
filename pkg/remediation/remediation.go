package remediation

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mikematt33/gh-inspect/pkg/models"
)

type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in-progress"
	StatusResolved   Status = "resolved"
	StatusAccepted   Status = "accepted"
	StatusIgnored    Status = "ignored"
)

type Entry struct {
	ID          string    `json:"id"`
	RepoName    string    `json:"repo_name,omitempty"`
	Analyzer    string    `json:"analyzer,omitempty"`
	FindingType string    `json:"finding_type,omitempty"`
	Message     string    `json:"message,omitempty"`
	Location    string    `json:"location,omitempty"`
	Status      Status    `json:"status"`
	Note        string    `json:"note,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Store struct {
	Entries map[string]Entry `json:"entries"`
}

func GetDefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".gh-inspect/remediation.json"
	}
	return filepath.Join(home, ".gh-inspect", "remediation.json")
}

func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{Entries: map[string]Entry{}}, nil
		}
		return nil, fmt.Errorf("failed to read remediation store: %w", err)
	}

	store := &Store{}
	if err := json.Unmarshal(data, store); err != nil {
		return nil, fmt.Errorf("failed to unmarshal remediation store: %w", err)
	}
	if store.Entries == nil {
		store.Entries = map[string]Entry{}
	}
	return store, nil
}

func Save(path string, store *Store) error {
	if store == nil {
		store = &Store{Entries: map[string]Entry{}}
	}
	if store.Entries == nil {
		store.Entries = map[string]Entry{}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create remediation directory: %w", err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal remediation store: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write remediation store: %w", err)
	}
	return nil
}

func IsValidStatus(status string) bool {
	switch Status(status) {
	case StatusOpen, StatusInProgress, StatusResolved, StatusAccepted, StatusIgnored:
		return true
	default:
		return false
	}
}

func SetStatus(store *Store, id string, status Status, note string) {
	if store == nil {
		return
	}
	if store.Entries == nil {
		store.Entries = map[string]Entry{}
	}
	entry := store.Entries[id]
	entry.ID = id
	entry.Status = status
	entry.Note = note
	entry.UpdatedAt = time.Now()
	store.Entries[id] = entry
}

func ListEntries(store *Store) []Entry {
	if store == nil {
		return nil
	}
	entries := make([]Entry, 0, len(store.Entries))
	for _, entry := range store.Entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].UpdatedAt.After(entries[j].UpdatedAt)
	})
	return entries
}

func ApplyReport(report *models.Report, store *Store) {
	if report == nil {
		return
	}
	if store == nil {
		store = &Store{Entries: map[string]Entry{}}
	}
	if store.Entries == nil {
		store.Entries = map[string]Entry{}
	}

	report.Summary.RemediationOpen = 0
	report.Summary.RemediationInProgress = 0
	report.Summary.RemediationResolved = 0
	report.Summary.RemediationAccepted = 0
	report.Summary.RemediationIgnored = 0

	for repoIdx := range report.Repositories {
		repo := &report.Repositories[repoIdx]
		for analyzerIdx := range repo.Analyzers {
			analyzer := &repo.Analyzers[analyzerIdx]
			for findingIdx := range analyzer.Findings {
				finding := &analyzer.Findings[findingIdx]
				id := FindingID(repo.Name, analyzer.Name, *finding)
				finding.TrackingID = id

				status := StatusOpen
				note := ""
				if entry, ok := store.Entries[id]; ok {
					status = entry.Status
					note = entry.Note
				} else {
					store.Entries[id] = Entry{
						ID:          id,
						RepoName:    repo.Name,
						Analyzer:    analyzer.Name,
						FindingType: finding.Type,
						Message:     finding.Message,
						Location:    finding.Location,
						Status:      StatusOpen,
					}
				}

				finding.RemediationState = string(status)
				finding.RemediationNote = note

				switch status {
				case StatusInProgress:
					report.Summary.RemediationInProgress++
				case StatusResolved:
					report.Summary.RemediationResolved++
				case StatusAccepted:
					report.Summary.RemediationAccepted++
				case StatusIgnored:
					report.Summary.RemediationIgnored++
				default:
					report.Summary.RemediationOpen++
				}
			}
		}
	}
}

func FindingID(repoName, analyzerName string, finding models.Finding) string {
	payload := strings.Join([]string{repoName, analyzerName, finding.Type, finding.Message, finding.Location}, "|")
	hash := sha1.Sum([]byte(payload))
	return hex.EncodeToString(hash[:])[:12]
}
