// Package messaging provides high-performance message bus implementation.
package messaging

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
	"github.com/google/uuid"
)

// MessageHandler is a callback function for handling messages.
type MessageHandler func(message *shared.BusMessage)

// Subscription represents a message subscription.
type Subscription struct {
	AgentID  string
	Callback MessageHandler
	Filter   []shared.BusMessageType // If empty, receives all message types
}

// PendingAck represents a pending acknowledgment.
type PendingAck struct {
	Message   *shared.BusMessage
	Timer     *time.Timer
	Timestamp int64
}

// MessageBus is a high-performance message bus with priority queues.
type MessageBus struct {
	config        shared.MessageBusConfig
	queues        map[string]*PriorityQueue // Per-agent queues
	subscriptions map[string]*Subscription
	pendingAcks   map[string]*PendingAck
	stats         *shared.MessageBusStats
	mu            sync.RWMutex
	ackMu         sync.RWMutex

	// Message counter for ID generation
	messageCounter atomic.Int64

	// Processing control
	ctx             context.Context
	cancel          context.CancelFunc
	processingTicker *time.Ticker
	statsTicker     *time.Ticker

	// Stats tracking
	messageHistory     []messageHistoryEntry
	messageHistoryIdx  int
	maxHistorySize     int

	// Callbacks for events
	onEnqueued  func(messageID, to string)
	onDelivered func(messageID, to string)
	onExpired   func(messageID string)
	onFailed    func(messageID string, err error)
}

type messageHistoryEntry struct {
	Timestamp int64
	Count     int64
}

// NewMessageBus creates a new MessageBus with the given configuration.
func NewMessageBus(config shared.MessageBusConfig) *MessageBus {
	ctx, cancel := context.WithCancel(context.Background())

	mb := &MessageBus{
		config:         config,
		queues:         make(map[string]*PriorityQueue),
		subscriptions:  make(map[string]*Subscription),
		pendingAcks:    make(map[string]*PendingAck),
		stats:          &shared.MessageBusStats{AckRate: 1.0},
		ctx:            ctx,
		cancel:         cancel,
		messageHistory: make([]messageHistoryEntry, 0),
		maxHistorySize: 60,
	}

	return mb
}

// NewMessageBusWithDefaults creates a new MessageBus with default configuration.
func NewMessageBusWithDefaults() *MessageBus {
	return NewMessageBus(shared.DefaultMessageBusConfig())
}

// Initialize starts the message bus processing loops.
func (mb *MessageBus) Initialize() error {
	mb.startProcessing()
	mb.startStatsCollection()
	return nil
}

// Shutdown stops the message bus and cleans up resources.
func (mb *MessageBus) Shutdown() error {
	mb.cancel()

	if mb.processingTicker != nil {
		mb.processingTicker.Stop()
	}
	if mb.statsTicker != nil {
		mb.statsTicker.Stop()
	}

	// Clear pending acks
	mb.ackMu.Lock()
	for _, pending := range mb.pendingAcks {
		if pending.Timer != nil {
			pending.Timer.Stop()
		}
	}
	mb.pendingAcks = make(map[string]*PendingAck)
	mb.ackMu.Unlock()

	// Clear queues
	mb.mu.Lock()
	mb.queues = make(map[string]*PriorityQueue)
	mb.subscriptions = make(map[string]*Subscription)
	mb.mu.Unlock()

	return nil
}

// generateMessageID generates a unique message ID.
func (mb *MessageBus) generateMessageID() string {
	count := mb.messageCounter.Add(1)
	return fmt.Sprintf("msg_%d_%s", count, uuid.New().String()[:8])
}

// Send sends a message to a specific agent.
func (mb *MessageBus) Send(msg shared.BusMessage) (string, error) {
	// Set message ID and timestamp
	if msg.ID == "" {
		msg.ID = mb.generateMessageID()
	}
	if msg.Timestamp == 0 {
		msg.Timestamp = shared.Now()
	}
	if msg.TTLMs == 0 {
		msg.TTLMs = mb.config.DefaultTTLMs
	}

	return mb.enqueue(&msg)
}

// Broadcast sends a message to all subscribed agents except the sender.
func (mb *MessageBus) Broadcast(msg shared.BusMessage) (string, error) {
	if msg.ID == "" {
		msg.ID = mb.generateMessageID()
	}
	if msg.Timestamp == 0 {
		msg.Timestamp = shared.Now()
	}
	if msg.TTLMs == 0 {
		msg.TTLMs = mb.config.DefaultTTLMs
	}
	msg.To = "broadcast"

	mb.mu.RLock()
	agents := make([]string, 0, len(mb.subscriptions))
	for agentID := range mb.subscriptions {
		if agentID != msg.From {
			agents = append(agents, agentID)
		}
	}
	mb.mu.RUnlock()

	for _, agentID := range agents {
		msgCopy := msg
		msgCopy.To = agentID
		mb.addToQueue(agentID, &msgCopy)
	}

	atomic.AddInt64(&mb.stats.TotalMessages, 1)

	if mb.onEnqueued != nil {
		mb.onEnqueued(msg.ID, "broadcast")
	}

	return msg.ID, nil
}

// enqueue adds a message to the appropriate queue.
func (mb *MessageBus) enqueue(msg *shared.BusMessage) (string, error) {
	if msg.To == "broadcast" {
		return mb.Broadcast(*msg)
	}

	mb.addToQueue(msg.To, msg)
	atomic.AddInt64(&mb.stats.TotalMessages, 1)

	if mb.onEnqueued != nil {
		mb.onEnqueued(msg.ID, msg.To)
	}

	return msg.ID, nil
}

// addToQueue adds a message to an agent's queue.
func (mb *MessageBus) addToQueue(agentID string, msg *shared.BusMessage) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	queue, exists := mb.queues[agentID]
	if !exists {
		queue = NewPriorityQueue()
		mb.queues[agentID] = queue
	}

	// Check queue size limit - remove lowest priority if full
	if queue.Len() >= mb.config.MaxQueueSize {
		removed := queue.RemoveLowestPriority()
		if removed != nil {
			atomic.AddInt64(&mb.stats.DroppedMessages, 1)
		}
	}

	entry := &shared.MessageEntry{
		Message:    msg,
		Attempts:   0,
		EnqueuedAt: shared.Now(),
	}

	queue.Enqueue(entry)
}

// Subscribe registers an agent to receive messages.
func (mb *MessageBus) Subscribe(agentID string, callback MessageHandler, filter []shared.BusMessageType) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.subscriptions[agentID] = &Subscription{
		AgentID:  agentID,
		Callback: callback,
		Filter:   filter,
	}

	// Initialize queue for this agent
	if _, exists := mb.queues[agentID]; !exists {
		mb.queues[agentID] = NewPriorityQueue()
	}
}

// Unsubscribe removes an agent's subscription.
func (mb *MessageBus) Unsubscribe(agentID string) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	delete(mb.subscriptions, agentID)
	delete(mb.queues, agentID)
}

// Acknowledge processes an acknowledgment for a message.
func (mb *MessageBus) Acknowledge(ack shared.MessageAck) error {
	mb.ackMu.Lock()
	defer mb.ackMu.Unlock()

	pending, exists := mb.pendingAcks[ack.MessageID]
	if !exists {
		return nil
	}

	if pending.Timer != nil {
		pending.Timer.Stop()
	}
	delete(mb.pendingAcks, ack.MessageID)

	if !ack.Received && ack.Error != "" {
		mb.handleAckFailure(pending.Message, ack.Error)
	}

	return nil
}

// GetMessages retrieves and removes all pending messages for an agent (pull mode).
func (mb *MessageBus) GetMessages(agentID string) []*shared.BusMessage {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	queue, exists := mb.queues[agentID]
	if !exists || queue.Len() == 0 {
		return nil
	}

	now := shared.Now()
	messages := make([]*shared.BusMessage, 0)

	for queue.Len() > 0 {
		entry := queue.Dequeue()
		if entry == nil {
			break
		}

		// Check TTL
		if now-entry.Message.Timestamp <= entry.Message.TTLMs {
			messages = append(messages, entry.Message)
		} else if mb.onExpired != nil {
			mb.onExpired(entry.Message.ID)
		}
	}

	return messages
}

// HasPendingMessages checks if an agent has pending messages.
func (mb *MessageBus) HasPendingMessages(agentID string) bool {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	queue, exists := mb.queues[agentID]
	return exists && queue.Len() > 0
}

// GetMessage retrieves a specific message by ID without removing it.
func (mb *MessageBus) GetMessage(messageID string) (*shared.BusMessage, bool) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	for _, queue := range mb.queues {
		entry, found := queue.FindByMessageID(messageID)
		if found {
			return entry.Message, true
		}
	}
	return nil, false
}

// GetStats returns the current message bus statistics.
func (mb *MessageBus) GetStats() shared.MessageBusStats {
	return *mb.stats
}

// GetQueueDepth returns the total number of queued messages.
func (mb *MessageBus) GetQueueDepth() int {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	total := 0
	for _, queue := range mb.queues {
		total += queue.Len()
	}
	return total
}

// startProcessing starts the message processing loop.
func (mb *MessageBus) startProcessing() {
	mb.processingTicker = time.NewTicker(time.Duration(mb.config.ProcessingIntervalMs) * time.Millisecond)

	go func() {
		for {
			select {
			case <-mb.ctx.Done():
				return
			case <-mb.processingTicker.C:
				mb.processQueues()
			}
		}
	}()
}

// processQueues processes messages in all queues.
func (mb *MessageBus) processQueues() {
	now := shared.Now()

	mb.mu.RLock()
	agentIDs := make([]string, 0, len(mb.queues))
	for agentID := range mb.queues {
		agentIDs = append(agentIDs, agentID)
	}
	mb.mu.RUnlock()

	for _, agentID := range agentIDs {
		mb.mu.RLock()
		subscription, hasSub := mb.subscriptions[agentID]
		queue, hasQueue := mb.queues[agentID]
		mb.mu.RUnlock()

		if !hasSub || !hasQueue {
			continue
		}

		// Process up to BatchSize messages per agent
		entries := queue.DequeueN(mb.config.BatchSize)

		for _, entry := range entries {
			// Check TTL
			if now-entry.Message.Timestamp > entry.Message.TTLMs {
				if mb.onExpired != nil {
					mb.onExpired(entry.Message.ID)
				}
				continue
			}

			// Check filter
			if len(subscription.Filter) > 0 {
				matched := false
				for _, t := range subscription.Filter {
					if t == entry.Message.Type {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}

			// Deliver message
			mb.deliverMessage(subscription, entry)
		}
	}
}

// deliverMessage delivers a message to a subscriber.
func (mb *MessageBus) deliverMessage(subscription *Subscription, entry *shared.MessageEntry) {
	msg := entry.Message

	// Set up ack timeout if required
	if msg.RequiresAck {
		mb.ackMu.Lock()
		timer := time.AfterFunc(time.Duration(mb.config.AckTimeoutMs)*time.Millisecond, func() {
			mb.handleAckTimeout(msg)
		})
		mb.pendingAcks[msg.ID] = &PendingAck{
			Message:   msg,
			Timer:     timer,
			Timestamp: shared.Now(),
		}
		mb.ackMu.Unlock()
	}

	// Deliver asynchronously
	go func() {
		defer func() {
			if r := recover(); r != nil {
				mb.handleDeliveryError(msg, entry, fmt.Errorf("panic: %v", r))
			}
		}()

		subscription.Callback(msg)

		if mb.onDelivered != nil {
			mb.onDelivered(msg.ID, subscription.AgentID)
		}
	}()
}

// handleAckTimeout handles an acknowledgment timeout.
func (mb *MessageBus) handleAckTimeout(msg *shared.BusMessage) {
	mb.ackMu.Lock()
	delete(mb.pendingAcks, msg.ID)
	mb.ackMu.Unlock()

	// Decrease ack rate
	currentRate := mb.stats.AckRate
	if currentRate > 0.01 {
		mb.stats.AckRate = currentRate - 0.01
	}
}

// handleAckFailure handles an acknowledgment failure.
func (mb *MessageBus) handleAckFailure(msg *shared.BusMessage, errStr string) {
	// Increase error rate
	currentRate := mb.stats.ErrorRate
	if currentRate < 1.0 {
		mb.stats.ErrorRate = currentRate + 0.01
	}

	if mb.onFailed != nil {
		mb.onFailed(msg.ID, fmt.Errorf(errStr))
	}
}

// handleDeliveryError handles a message delivery error.
func (mb *MessageBus) handleDeliveryError(msg *shared.BusMessage, entry *shared.MessageEntry, err error) {
	entry.Attempts++
	entry.LastAttemptAt = shared.Now()

	if entry.Attempts < mb.config.RetryAttempts {
		// Re-queue for retry
		mb.addToQueue(msg.To, msg)
	} else {
		// Max retries exceeded
		currentRate := mb.stats.ErrorRate
		if currentRate < 1.0 {
			mb.stats.ErrorRate = currentRate + 0.01
		}

		if mb.onFailed != nil {
			mb.onFailed(msg.ID, err)
		}
	}
}

// startStatsCollection starts the stats collection loop.
func (mb *MessageBus) startStatsCollection() {
	mb.statsTicker = time.NewTicker(1 * time.Second)

	go func() {
		for {
			select {
			case <-mb.ctx.Done():
				return
			case <-mb.statsTicker.C:
				mb.calculateMessagesPerSecond()
			}
		}
	}()
}

// calculateMessagesPerSecond calculates the messages per second rate.
func (mb *MessageBus) calculateMessagesPerSecond() {
	now := shared.Now()
	count := mb.stats.TotalMessages

	entry := messageHistoryEntry{
		Timestamp: now,
		Count:     count,
	}

	// Use circular buffer pattern
	if len(mb.messageHistory) < mb.maxHistorySize {
		mb.messageHistory = append(mb.messageHistory, entry)
	} else {
		mb.messageHistory[mb.messageHistoryIdx] = entry
		mb.messageHistoryIdx = (mb.messageHistoryIdx + 1) % mb.maxHistorySize
	}

	// Calculate messages per second from history
	if len(mb.messageHistory) >= 2 {
		var oldest messageHistoryEntry
		for _, h := range mb.messageHistory {
			if oldest.Timestamp == 0 || (h.Timestamp < oldest.Timestamp && now-h.Timestamp < 60000) {
				oldest = h
			}
		}

		if oldest.Timestamp > 0 {
			seconds := float64(now-oldest.Timestamp) / 1000.0
			messages := float64(count - oldest.Count)
			if seconds > 0 {
				mb.stats.MessagesPerSecond = messages / seconds
			}
		}
	}

	// Update queue depth
	mb.stats.QueueDepth = mb.GetQueueDepth()
}

// SetOnEnqueued sets the callback for when a message is enqueued.
func (mb *MessageBus) SetOnEnqueued(callback func(messageID, to string)) {
	mb.onEnqueued = callback
}

// SetOnDelivered sets the callback for when a message is delivered.
func (mb *MessageBus) SetOnDelivered(callback func(messageID, to string)) {
	mb.onDelivered = callback
}

// SetOnExpired sets the callback for when a message expires.
func (mb *MessageBus) SetOnExpired(callback func(messageID string)) {
	mb.onExpired = callback
}

// SetOnFailed sets the callback for when a message delivery fails.
func (mb *MessageBus) SetOnFailed(callback func(messageID string, err error)) {
	mb.onFailed = callback
}

// GetConfig returns the current configuration.
func (mb *MessageBus) GetConfig() shared.MessageBusConfig {
	return mb.config
}

// GetSubscriptionCount returns the number of subscriptions.
func (mb *MessageBus) GetSubscriptionCount() int {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	return len(mb.subscriptions)
}

// IsSubscribed checks if an agent is subscribed.
func (mb *MessageBus) IsSubscribed(agentID string) bool {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	_, exists := mb.subscriptions[agentID]
	return exists
}
