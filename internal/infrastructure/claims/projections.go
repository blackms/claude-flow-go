// Package claims provides infrastructure for the claims system.
package claims

import (
	"sync"
	"time"

	domainClaims "github.com/anthropics/claude-flow-go/internal/domain/claims"
)

// Projection defines the interface for event projections.
type Projection interface {
	Apply(event *domainClaims.ClaimEvent)
	Reset()
}

// ClaimSummary is a read model for claim summaries.
type ClaimSummary struct {
	ID             string                     `json:"id"`
	IssueID        string                     `json:"issueId"`
	ClaimantID     string                     `json:"claimantId"`
	ClaimantName   string                     `json:"claimantName"`
	Status         domainClaims.ClaimStatus   `json:"status"`
	Progress       float64                    `json:"progress"`
	ClaimedAt      time.Time                  `json:"claimedAt"`
	LastActivityAt time.Time                  `json:"lastActivityAt"`
	HandoffCount   int                        `json:"handoffCount"`
}

// ClaimantStats is a read model for claimant statistics.
type ClaimantStats struct {
	ClaimantID        string    `json:"claimantId"`
	TotalClaims       int       `json:"totalClaims"`
	ActiveClaims      int       `json:"activeClaims"`
	CompletedClaims   int       `json:"completedClaims"`
	ReleasedClaims    int       `json:"releasedClaims"`
	HandoffsInitiated int       `json:"handoffsInitiated"`
	HandoffsReceived  int       `json:"handoffsReceived"`
	LastActivityAt    time.Time `json:"lastActivityAt"`
}

// SystemStats is a read model for system-wide statistics.
type SystemStats struct {
	TotalClaims     int       `json:"totalClaims"`
	ActiveClaims    int       `json:"activeClaims"`
	CompletedClaims int       `json:"completedClaims"`
	StealableClaims int       `json:"stealableClaims"`
	TotalHandoffs   int       `json:"totalHandoffs"`
	TotalSteals     int       `json:"totalSteals"`
	LastEventAt     time.Time `json:"lastEventAt"`
}

// ClaimSummaryProjection maintains claim summary read models.
type ClaimSummaryProjection struct {
	mu       sync.RWMutex
	summaries map[string]*ClaimSummary
}

// NewClaimSummaryProjection creates a new claim summary projection.
func NewClaimSummaryProjection() *ClaimSummaryProjection {
	return &ClaimSummaryProjection{
		summaries: make(map[string]*ClaimSummary),
	}
}

// Apply applies an event to the projection.
func (p *ClaimSummaryProjection) Apply(event *domainClaims.ClaimEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	claimID := event.AggregateID

	switch event.Type {
	case domainClaims.EventClaimCreated:
		p.summaries[claimID] = &ClaimSummary{
			ID:             claimID,
			IssueID:        getString(event.Payload, "issueId"),
			ClaimantID:     getString(event.Payload, "claimantId"),
			Status:         domainClaims.StatusActive,
			Progress:       0,
			ClaimedAt:      event.Timestamp,
			LastActivityAt: event.Timestamp,
			HandoffCount:   0,
		}

		// Extract claimant name if available
		if claimant, ok := event.Payload["claimant"].(domainClaims.Claimant); ok {
			p.summaries[claimID].ClaimantName = claimant.Name
		}

	case domainClaims.EventClaimStatusChanged:
		if summary, exists := p.summaries[claimID]; exists {
			if newStatus, ok := event.Payload["newStatus"].(domainClaims.ClaimStatus); ok {
				summary.Status = newStatus
			}
			summary.LastActivityAt = event.Timestamp
		}

	case domainClaims.EventClaimProgressUpdated:
		if summary, exists := p.summaries[claimID]; exists {
			if progress, ok := event.Payload["progress"].(float64); ok {
				summary.Progress = progress
			}
			summary.LastActivityAt = event.Timestamp
		}

	case domainClaims.EventClaimCompleted:
		if summary, exists := p.summaries[claimID]; exists {
			summary.Status = domainClaims.StatusCompleted
			summary.Progress = 1.0
			summary.LastActivityAt = event.Timestamp
		}

	case domainClaims.EventClaimReleased:
		if summary, exists := p.summaries[claimID]; exists {
			summary.Status = domainClaims.StatusReleased
			summary.LastActivityAt = event.Timestamp
		}

	case domainClaims.EventHandoffAccepted:
		if summary, exists := p.summaries[claimID]; exists {
			summary.HandoffCount++
			summary.LastActivityAt = event.Timestamp
		}

	case domainClaims.EventIssueStolen:
		if summary, exists := p.summaries[claimID]; exists {
			if newClaimantID, ok := event.Payload["newClaimantId"].(string); ok {
				summary.ClaimantID = newClaimantID
			}
			summary.Status = domainClaims.StatusActive
			summary.LastActivityAt = event.Timestamp
		}
	}
}

// Reset resets the projection.
func (p *ClaimSummaryProjection) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.summaries = make(map[string]*ClaimSummary)
}

// Get returns a claim summary by ID.
func (p *ClaimSummaryProjection) Get(claimID string) (*ClaimSummary, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	summary, exists := p.summaries[claimID]
	if !exists {
		return nil, false
	}
	summaryCopy := *summary
	return &summaryCopy, true
}

// GetAll returns all claim summaries.
func (p *ClaimSummaryProjection) GetAll() []*ClaimSummary {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*ClaimSummary, 0, len(p.summaries))
	for _, summary := range p.summaries {
		summaryCopy := *summary
		result = append(result, &summaryCopy)
	}
	return result
}

// GetByClaimant returns claim summaries for a claimant.
func (p *ClaimSummaryProjection) GetByClaimant(claimantID string) []*ClaimSummary {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*ClaimSummary, 0)
	for _, summary := range p.summaries {
		if summary.ClaimantID == claimantID {
			summaryCopy := *summary
			result = append(result, &summaryCopy)
		}
	}
	return result
}

// GetActive returns active claim summaries.
func (p *ClaimSummaryProjection) GetActive() []*ClaimSummary {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*ClaimSummary, 0)
	for _, summary := range p.summaries {
		if summary.Status.IsActive() {
			summaryCopy := *summary
			result = append(result, &summaryCopy)
		}
	}
	return result
}

// ClaimantStatsProjection maintains claimant statistics read models.
type ClaimantStatsProjection struct {
	mu    sync.RWMutex
	stats map[string]*ClaimantStats
}

// NewClaimantStatsProjection creates a new claimant stats projection.
func NewClaimantStatsProjection() *ClaimantStatsProjection {
	return &ClaimantStatsProjection{
		stats: make(map[string]*ClaimantStats),
	}
}

// Apply applies an event to the projection.
func (p *ClaimantStatsProjection) Apply(event *domainClaims.ClaimEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch event.Type {
	case domainClaims.EventClaimCreated:
		claimantID := getString(event.Payload, "claimantId")
		stats := p.getOrCreate(claimantID)
		stats.TotalClaims++
		stats.ActiveClaims++
		stats.LastActivityAt = event.Timestamp

	case domainClaims.EventClaimCompleted:
		// Need to look up the claim to find claimant
		// This is a simplified version - in practice, we'd cache this
		claimID := event.AggregateID
		for cID, s := range p.stats {
			if s.ClaimantID == cID {
				s.ActiveClaims--
				s.CompletedClaims++
				s.LastActivityAt = event.Timestamp
				break
			}
			_ = claimID // suppress unused warning
		}

	case domainClaims.EventClaimReleased:
		for _, s := range p.stats {
			if s.ActiveClaims > 0 {
				s.ActiveClaims--
				s.ReleasedClaims++
				s.LastActivityAt = event.Timestamp
			}
		}

	case domainClaims.EventHandoffRequested:
		fromID := getString(event.Payload, "fromId")
		if stats, exists := p.stats[fromID]; exists {
			stats.HandoffsInitiated++
			stats.LastActivityAt = event.Timestamp
		}

	case domainClaims.EventHandoffAccepted:
		toID := getString(event.Payload, "toId")
		if toID != "" {
			stats := p.getOrCreate(toID)
			stats.HandoffsReceived++
			stats.ActiveClaims++
			stats.LastActivityAt = event.Timestamp
		}
	}
}

// getOrCreate returns existing stats or creates new ones.
func (p *ClaimantStatsProjection) getOrCreate(claimantID string) *ClaimantStats {
	if stats, exists := p.stats[claimantID]; exists {
		return stats
	}
	stats := &ClaimantStats{ClaimantID: claimantID}
	p.stats[claimantID] = stats
	return stats
}

// Reset resets the projection.
func (p *ClaimantStatsProjection) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats = make(map[string]*ClaimantStats)
}

// Get returns stats for a claimant.
func (p *ClaimantStatsProjection) Get(claimantID string) (*ClaimantStats, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	stats, exists := p.stats[claimantID]
	if !exists {
		return nil, false
	}
	statsCopy := *stats
	return &statsCopy, true
}

// GetAll returns all claimant stats.
func (p *ClaimantStatsProjection) GetAll() []*ClaimantStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*ClaimantStats, 0, len(p.stats))
	for _, stats := range p.stats {
		statsCopy := *stats
		result = append(result, &statsCopy)
	}
	return result
}

// SystemStatsProjection maintains system-wide statistics.
type SystemStatsProjection struct {
	mu    sync.RWMutex
	stats SystemStats
}

// NewSystemStatsProjection creates a new system stats projection.
func NewSystemStatsProjection() *SystemStatsProjection {
	return &SystemStatsProjection{
		stats: SystemStats{},
	}
}

// Apply applies an event to the projection.
func (p *SystemStatsProjection) Apply(event *domainClaims.ClaimEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.LastEventAt = event.Timestamp

	switch event.Type {
	case domainClaims.EventClaimCreated:
		p.stats.TotalClaims++
		p.stats.ActiveClaims++

	case domainClaims.EventClaimCompleted:
		p.stats.ActiveClaims--
		p.stats.CompletedClaims++

	case domainClaims.EventClaimReleased:
		p.stats.ActiveClaims--

	case domainClaims.EventIssueMarkedStealable:
		p.stats.StealableClaims++

	case domainClaims.EventIssueStolen:
		p.stats.StealableClaims--
		p.stats.TotalSteals++

	case domainClaims.EventHandoffAccepted:
		p.stats.TotalHandoffs++
	}
}

// Reset resets the projection.
func (p *SystemStatsProjection) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats = SystemStats{}
}

// Get returns the current system stats.
func (p *SystemStatsProjection) Get() SystemStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// ProjectionManager manages multiple projections.
type ProjectionManager struct {
	projections []Projection
	eventStore  EventStore
}

// NewProjectionManager creates a new projection manager.
func NewProjectionManager(eventStore EventStore) *ProjectionManager {
	return &ProjectionManager{
		projections: make([]Projection, 0),
		eventStore:  eventStore,
	}
}

// Register registers a projection.
func (m *ProjectionManager) Register(projection Projection) {
	m.projections = append(m.projections, projection)
}

// RebuildAll rebuilds all projections from the event store.
func (m *ProjectionManager) RebuildAll() error {
	// Reset all projections
	for _, p := range m.projections {
		p.Reset()
	}

	// Replay all events
	events, err := m.eventStore.GetAllEvents()
	if err != nil {
		return err
	}

	for _, event := range events {
		for _, p := range m.projections {
			p.Apply(event)
		}
	}

	return nil
}

// Subscribe subscribes all projections to new events.
func (m *ProjectionManager) Subscribe() {
	m.eventStore.SubscribeAll(func(event *domainClaims.ClaimEvent) {
		for _, p := range m.projections {
			p.Apply(event)
		}
	})
}

// Helper function to get string from payload.
func getString(payload map[string]interface{}, key string) string {
	if v, ok := payload[key].(string); ok {
		return v
	}
	return ""
}
