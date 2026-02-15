// Package federation provides cross-swarm coordination and ephemeral agent management.
package federation

import (
	"fmt"
	"strings"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// ============================================================================
// Cross-Swarm Messaging
// ============================================================================

// SendMessage sends a direct message to a specific swarm.
func (fh *FederationHub) SendMessage(sourceSwarmID, targetSwarmID string, payload interface{}) (*shared.FederationMessage, error) {
	startTime := shared.Now()

	fh.mu.Lock()
	defer fh.mu.Unlock()

	sourceSwarmID = strings.TrimSpace(sourceSwarmID)
	targetSwarmID = strings.TrimSpace(targetSwarmID)
	if sourceSwarmID == "" {
		return nil, fmt.Errorf("sourceSwarmId is required")
	}
	if targetSwarmID == "" {
		return nil, fmt.Errorf("targetSwarmId is required")
	}
	if payload == nil {
		return nil, fmt.Errorf("payload is required")
	}

	// Validate source swarm
	sourceSwarm, exists := fh.swarms[sourceSwarmID]
	if !exists {
		return nil, fmt.Errorf("source swarm %s not found", sourceSwarmID)
	}
	if sourceSwarm.Status != shared.SwarmStatusActive {
		return nil, fmt.Errorf("source swarm %s is not active", sourceSwarmID)
	}

	// Validate target swarm
	targetSwarm, exists := fh.swarms[targetSwarmID]
	if !exists {
		return nil, fmt.Errorf("target swarm %s not found", targetSwarmID)
	}
	if targetSwarm.Status == shared.SwarmStatusInactive {
		return nil, fmt.Errorf("target swarm %s is inactive", targetSwarmID)
	}

	now := shared.Now()
	msg := &shared.FederationMessage{
		ID:            generateID("msg"),
		Type:          shared.FederationMsgDirect,
		SourceSwarmID: sourceSwarmID,
		TargetSwarmID: targetSwarmID,
		Payload:       payload,
		Timestamp:     now,
	}

	// Add to message history
	fh.addMessage(msg)

	// Update stats
	fh.messageCount.Add(1)

	// Emit events
	fh.emitEvent(shared.FederationEvent{
		Type:      shared.FederationEventMessageSent,
		SwarmID:   sourceSwarmID,
		Data:      map[string]interface{}{"messageId": msg.ID, "target": targetSwarmID},
		Timestamp: now,
	})

	fh.emitEvent(shared.FederationEvent{
		Type:      shared.FederationEventMessageReceived,
		SwarmID:   targetSwarmID,
		Data:      map[string]interface{}{"messageId": msg.ID, "source": sourceSwarmID},
		Timestamp: now,
	})

	// Record message time
	deliveryTime := shared.Now() - startTime
	fh.recordMessageTime(deliveryTime)

	return cloneFederationMessage(msg), nil
}

// Broadcast sends a message to all active swarms except the sender.
func (fh *FederationHub) Broadcast(sourceSwarmID string, payload interface{}) (*shared.FederationMessage, error) {
	startTime := shared.Now()

	fh.mu.Lock()
	defer fh.mu.Unlock()

	sourceSwarmID = strings.TrimSpace(sourceSwarmID)
	if sourceSwarmID == "" {
		return nil, fmt.Errorf("sourceSwarmId is required")
	}
	if payload == nil {
		return nil, fmt.Errorf("payload is required")
	}

	// Validate source swarm
	sourceSwarm, exists := fh.swarms[sourceSwarmID]
	if !exists {
		return nil, fmt.Errorf("source swarm %s not found", sourceSwarmID)
	}
	if sourceSwarm.Status != shared.SwarmStatusActive {
		return nil, fmt.Errorf("source swarm %s is not active", sourceSwarmID)
	}

	now := shared.Now()
	msg := &shared.FederationMessage{
		ID:            generateID("broadcast"),
		Type:          shared.FederationMsgBroadcast,
		SourceSwarmID: sourceSwarmID,
		TargetSwarmID: "", // Empty for broadcast
		Payload:       payload,
		Timestamp:     now,
	}

	// Add to message history
	fh.addMessage(msg)

	// Update stats
	fh.messageCount.Add(1)

	// Emit send event
	fh.emitEvent(shared.FederationEvent{
		Type:      shared.FederationEventMessageSent,
		SwarmID:   sourceSwarmID,
		Data:      map[string]interface{}{"messageId": msg.ID, "type": "broadcast"},
		Timestamp: now,
	})

	// Emit receive events for all active swarms
	for swarmID, swarm := range fh.swarms {
		if swarmID == sourceSwarmID {
			continue
		}
		if swarm.Status == shared.SwarmStatusActive || swarm.Status == shared.SwarmStatusDegraded {
			fh.emitEvent(shared.FederationEvent{
				Type:      shared.FederationEventMessageReceived,
				SwarmID:   swarmID,
				Data:      map[string]interface{}{"messageId": msg.ID, "source": sourceSwarmID, "type": "broadcast"},
				Timestamp: now,
			})
		}
	}

	// Record message time
	deliveryTime := shared.Now() - startTime
	fh.recordMessageTime(deliveryTime)

	return cloneFederationMessage(msg), nil
}

// SendHeartbeat sends a heartbeat message to a swarm.
func (fh *FederationHub) SendHeartbeat(sourceSwarmID, targetSwarmID string) (*shared.FederationMessage, error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	sourceSwarmID = strings.TrimSpace(sourceSwarmID)
	targetSwarmID = strings.TrimSpace(targetSwarmID)
	if sourceSwarmID == "" {
		return nil, fmt.Errorf("sourceSwarmId is required")
	}
	if targetSwarmID == "" {
		return nil, fmt.Errorf("targetSwarmId is required")
	}

	// Validate source swarm
	sourceSwarm, exists := fh.swarms[sourceSwarmID]
	if !exists {
		return nil, fmt.Errorf("source swarm %s not found", sourceSwarmID)
	}
	if sourceSwarm.Status != shared.SwarmStatusActive {
		return nil, fmt.Errorf("source swarm %s is not active", sourceSwarmID)
	}

	// Validate target swarm
	targetSwarm, exists := fh.swarms[targetSwarmID]
	if !exists {
		return nil, fmt.Errorf("target swarm %s not found", targetSwarmID)
	}
	if targetSwarm.Status == shared.SwarmStatusInactive {
		return nil, fmt.Errorf("target swarm %s is inactive", targetSwarmID)
	}

	now := shared.Now()
	msg := &shared.FederationMessage{
		ID:            generateID("heartbeat"),
		Type:          shared.FederationMsgHeartbeat,
		SourceSwarmID: sourceSwarmID,
		TargetSwarmID: targetSwarmID,
		Payload:       map[string]interface{}{"timestamp": now},
		Timestamp:     now,
	}

	// Add to message history
	fh.addMessage(msg)

	// Update heartbeat for source swarm
	sourceSwarm.LastHeartbeat = now

	return cloneFederationMessage(msg), nil
}

// SendConsensusMessage sends a consensus-related message.
func (fh *FederationHub) SendConsensusMessage(sourceSwarmID string, payload interface{}, targetSwarmID string) (*shared.FederationMessage, error) {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	sourceSwarmID = strings.TrimSpace(sourceSwarmID)
	targetSwarmID = strings.TrimSpace(targetSwarmID)
	if sourceSwarmID == "" {
		return nil, fmt.Errorf("sourceSwarmId is required")
	}
	if payload == nil {
		return nil, fmt.Errorf("payload is required")
	}

	// Validate source swarm
	sourceSwarm, exists := fh.swarms[sourceSwarmID]
	if !exists {
		return nil, fmt.Errorf("source swarm %s not found", sourceSwarmID)
	}
	if sourceSwarm.Status != shared.SwarmStatusActive {
		return nil, fmt.Errorf("source swarm %s is not active", sourceSwarmID)
	}

	// Validate target swarm if provided
	if targetSwarmID != "" {
		targetSwarm, exists := fh.swarms[targetSwarmID]
		if !exists {
			return nil, fmt.Errorf("target swarm %s not found", targetSwarmID)
		}
		if targetSwarm.Status == shared.SwarmStatusInactive {
			return nil, fmt.Errorf("target swarm %s is inactive", targetSwarmID)
		}
	}

	now := shared.Now()
	msg := &shared.FederationMessage{
		ID:            generateID("consensus"),
		Type:          shared.FederationMsgConsensus,
		SourceSwarmID: sourceSwarmID,
		TargetSwarmID: targetSwarmID,
		Payload:       payload,
		Timestamp:     now,
	}

	// Add to message history
	fh.addMessage(msg)

	fh.messageCount.Add(1)

	return cloneFederationMessage(msg), nil
}

// addMessage adds a message to the history.
func (fh *FederationHub) addMessage(msg *shared.FederationMessage) {
	fh.messages = append(fh.messages, msg)

	// Limit message history
	if len(fh.messages) > fh.config.MaxMessageHistory {
		fh.messages = fh.messages[1:]
	}
}

// GetMessages returns recent messages.
func (fh *FederationHub) GetMessages(limit int) []*shared.FederationMessage {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	if limit <= 0 || limit > len(fh.messages) {
		limit = len(fh.messages)
	}

	// Return most recent messages
	start := len(fh.messages) - limit
	result := make([]*shared.FederationMessage, limit)
	for i, msg := range fh.messages[start:] {
		result[i] = cloneFederationMessage(msg)
	}
	return result
}

// GetMessagesBySwarm returns messages for a specific swarm.
func (fh *FederationHub) GetMessagesBySwarm(swarmID string, limit int) []*shared.FederationMessage {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	swarmID = strings.TrimSpace(swarmID)
	if swarmID == "" {
		return []*shared.FederationMessage{}
	}
	if limit <= 0 || limit > len(fh.messages) {
		limit = len(fh.messages)
	}

	result := make([]*shared.FederationMessage, 0)

	// Iterate in reverse for most recent first
	for i := len(fh.messages) - 1; i >= 0 && len(result) < limit; i-- {
		msg := fh.messages[i]
		if msg.SourceSwarmID == swarmID || msg.TargetSwarmID == swarmID ||
			(msg.Type == shared.FederationMsgBroadcast && msg.TargetSwarmID == "") {
			result = append(result, cloneFederationMessage(msg))
		}
	}

	return result
}

// GetMessagesByType returns messages of a specific type.
func (fh *FederationHub) GetMessagesByType(msgType shared.FederationMessageType, limit int) []*shared.FederationMessage {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	if limit <= 0 || limit > len(fh.messages) {
		limit = len(fh.messages)
	}

	result := make([]*shared.FederationMessage, 0)

	// Iterate in reverse for most recent first
	for i := len(fh.messages) - 1; i >= 0 && len(result) < limit; i-- {
		msg := fh.messages[i]
		if msg.Type == msgType {
			result = append(result, cloneFederationMessage(msg))
		}
	}

	return result
}

// GetMessage returns a message by ID.
func (fh *FederationHub) GetMessage(messageID string) (*shared.FederationMessage, bool) {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return nil, false
	}

	for _, msg := range fh.messages {
		if msg.ID == messageID {
			return cloneFederationMessage(msg), true
		}
	}
	return nil, false
}
