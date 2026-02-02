// Package claims provides domain types for the claims system with event sourcing.
package claims

import (
	"time"
)

// Issue represents an issue that can be claimed.
type Issue struct {
	ID                   string     `json:"id"`
	Title                string     `json:"title"`
	Description          string     `json:"description"`
	Priority             Priority   `json:"priority"`
	Complexity           Complexity `json:"complexity"`
	Labels               []string   `json:"labels"`
	RequiredCapabilities []string   `json:"requiredCapabilities,omitempty"`
	RepositoryID         string     `json:"repositoryId,omitempty"`
	URL                  string     `json:"url,omitempty"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
}

// NewIssue creates a new issue.
func NewIssue(id, title, description string) *Issue {
	now := time.Now()
	return &Issue{
		ID:                   id,
		Title:                title,
		Description:          description,
		Priority:             PriorityMedium,
		Complexity:           ComplexityModerate,
		Labels:               make([]string, 0),
		RequiredCapabilities: make([]string, 0),
		CreatedAt:            now,
		UpdatedAt:            now,
		Metadata:             make(map[string]interface{}),
	}
}

// HasLabel checks if the issue has a specific label.
func (i *Issue) HasLabel(label string) bool {
	for _, l := range i.Labels {
		if l == label {
			return true
		}
	}
	return false
}

// AddLabel adds a label to the issue.
func (i *Issue) AddLabel(label string) {
	if !i.HasLabel(label) {
		i.Labels = append(i.Labels, label)
		i.UpdatedAt = time.Now()
	}
}

// RemoveLabel removes a label from the issue.
func (i *Issue) RemoveLabel(label string) {
	for idx, l := range i.Labels {
		if l == label {
			i.Labels = append(i.Labels[:idx], i.Labels[idx+1:]...)
			i.UpdatedAt = time.Now()
			return
		}
	}
}

// RequiresCapability checks if the issue requires a specific capability.
func (i *Issue) RequiresCapability(capability string) bool {
	for _, cap := range i.RequiredCapabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// EstimatedDuration returns the estimated duration based on complexity.
func (i *Issue) EstimatedDuration() time.Duration {
	hours := i.Complexity.EstimatedHours()
	return time.Duration(hours) * time.Hour
}

// IsCritical returns true if the issue is critical priority.
func (i *Issue) IsCritical() bool {
	return i.Priority == PriorityCritical
}

// IsHighPriority returns true if the issue is high or critical priority.
func (i *Issue) IsHighPriority() bool {
	return i.Priority == PriorityCritical || i.Priority == PriorityHigh
}
