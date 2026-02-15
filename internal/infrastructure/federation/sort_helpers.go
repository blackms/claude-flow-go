package federation

import (
	"sort"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func sortSwarmRegistrationsByID(swarms []*shared.SwarmRegistration) {
	sort.SliceStable(swarms, func(i, j int) bool {
		left := swarms[i]
		right := swarms[j]
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return left.SwarmID < right.SwarmID
	})
}

func sortEphemeralAgentsByID(agents []*shared.EphemeralAgent) {
	sort.SliceStable(agents, func(i, j int) bool {
		left := agents[i]
		right := agents[j]
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return left.ID < right.ID
	})
}

func sortFederationProposalsByID(proposals []*shared.FederationProposal) {
	sort.SliceStable(proposals, func(i, j int) bool {
		left := proposals[i]
		right := proposals[j]
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return left.ID < right.ID
	})
}
