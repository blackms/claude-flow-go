package federation

import (
	"github.com/anthropics/claude-flow-go/internal/shared"
)

func cloneSwarmRegistration(swarm *shared.SwarmRegistration) *shared.SwarmRegistration {
	if swarm == nil {
		return nil
	}

	cloned := *swarm
	cloned.Capabilities = append([]string(nil), swarm.Capabilities...)
	cloned.Metadata = shared.CloneStringInterfaceMap(swarm.Metadata)
	return &cloned
}

func cloneEphemeralAgent(agent *shared.EphemeralAgent) *shared.EphemeralAgent {
	if agent == nil {
		return nil
	}

	cloned := *agent
	cloned.Metadata = shared.CloneStringInterfaceMap(agent.Metadata)
	cloned.Result = shared.CloneInterfaceValue(agent.Result)
	return &cloned
}

func cloneFederationMessage(message *shared.FederationMessage) *shared.FederationMessage {
	if message == nil {
		return nil
	}

	cloned := *message
	cloned.Payload = shared.CloneInterfaceValue(message.Payload)
	return &cloned
}

func cloneFederationProposal(proposal *shared.FederationProposal) *shared.FederationProposal {
	if proposal == nil {
		return nil
	}

	cloned := *proposal
	cloned.Value = shared.CloneInterfaceValue(proposal.Value)
	if proposal.Votes != nil {
		cloned.Votes = make(map[string]bool, len(proposal.Votes))
		for voterID, approve := range proposal.Votes {
			cloned.Votes[voterID] = approve
		}
	}
	return &cloned
}

func cloneFederationEvent(event *shared.FederationEvent) *shared.FederationEvent {
	if event == nil {
		return nil
	}

	cloned := *event
	cloned.Data = shared.CloneInterfaceValue(event.Data)
	return &cloned
}
