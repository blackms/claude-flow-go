// Package claims provides domain types for the claims system with event sourcing.
package claims

// Claimant represents an entity that can claim issues.
type Claimant struct {
	ID              string       `json:"id"`
	Type            ClaimantType `json:"type"`
	Name            string       `json:"name"`
	Capabilities    []string     `json:"capabilities,omitempty"`
	Specializations []string     `json:"specializations,omitempty"`
	CurrentWorkload int          `json:"currentWorkload"`
	MaxConcurrent   int          `json:"maxConcurrentClaims"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// NewClaimant creates a new claimant.
func NewClaimant(id, name string, claimantType ClaimantType) *Claimant {
	maxConcurrent := 5
	if claimantType == ClaimantTypeAgent {
		maxConcurrent = 10
	}

	return &Claimant{
		ID:              id,
		Type:            claimantType,
		Name:            name,
		Capabilities:    make([]string, 0),
		Specializations: make([]string, 0),
		CurrentWorkload: 0,
		MaxConcurrent:   maxConcurrent,
		Metadata:        make(map[string]interface{}),
	}
}

// HasCapability checks if the claimant has a specific capability.
func (c *Claimant) HasCapability(capability string) bool {
	for _, cap := range c.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// HasAllCapabilities checks if the claimant has all required capabilities.
func (c *Claimant) HasAllCapabilities(required []string) bool {
	for _, req := range required {
		if !c.HasCapability(req) {
			return false
		}
	}
	return true
}

// CanAcceptMoreWork returns true if the claimant can accept more work.
func (c *Claimant) CanAcceptMoreWork() bool {
	return c.CurrentWorkload < c.MaxConcurrent
}

// Utilization returns the current utilization as a percentage.
func (c *Claimant) Utilization() float64 {
	if c.MaxConcurrent == 0 {
		return 0
	}
	return float64(c.CurrentWorkload) / float64(c.MaxConcurrent)
}

// IncrementWorkload increments the current workload.
func (c *Claimant) IncrementWorkload() {
	c.CurrentWorkload++
}

// DecrementWorkload decrements the current workload.
func (c *Claimant) DecrementWorkload() {
	if c.CurrentWorkload > 0 {
		c.CurrentWorkload--
	}
}

// IsHuman returns true if the claimant is human.
func (c *Claimant) IsHuman() bool {
	return c.Type == ClaimantTypeHuman
}

// IsAgent returns true if the claimant is an agent.
func (c *Claimant) IsAgent() bool {
	return c.Type == ClaimantTypeAgent
}
