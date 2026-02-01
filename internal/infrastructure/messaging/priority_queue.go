// Package messaging provides high-performance message bus implementation.
package messaging

import (
	"sync"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

// PriorityQueue is a priority queue using 4-level deques for O(1) operations.
type PriorityQueue struct {
	queues     [shared.MessagePriorityCount]*Deque[*shared.MessageEntry]
	totalCount int
	mu         sync.RWMutex
}

// NewPriorityQueue creates a new priority queue.
func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{}
	for i := 0; i < shared.MessagePriorityCount; i++ {
		pq.queues[i] = NewDeque[*shared.MessageEntry](16)
	}
	return pq
}

// Len returns the total number of elements across all priority levels.
func (pq *PriorityQueue) Len() int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()
	return pq.totalCount
}

// IsEmpty returns true if the queue is empty.
func (pq *PriorityQueue) IsEmpty() bool {
	return pq.Len() == 0
}

// Enqueue adds a message entry to the appropriate priority queue. O(1).
func (pq *PriorityQueue) Enqueue(entry *shared.MessageEntry) {
	if entry == nil || entry.Message == nil {
		return
	}

	pq.mu.Lock()
	defer pq.mu.Unlock()

	priority := entry.Message.Priority
	if priority < 0 || int(priority) >= shared.MessagePriorityCount {
		priority = shared.MessagePriorityNormal
	}

	pq.queues[priority].PushBack(entry)
	pq.totalCount++
}

// Dequeue removes and returns the highest priority message. O(1).
// Checks queues in priority order: Urgent, High, Normal, Low.
func (pq *PriorityQueue) Dequeue() *shared.MessageEntry {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Check from highest priority (0) to lowest (3)
	for i := 0; i < shared.MessagePriorityCount; i++ {
		if pq.queues[i].Len() > 0 {
			entry, ok := pq.queues[i].PopFront()
			if ok {
				pq.totalCount--
				return entry
			}
		}
	}
	return nil
}

// DequeueN removes and returns up to n highest priority messages.
func (pq *PriorityQueue) DequeueN(n int) []*shared.MessageEntry {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	result := make([]*shared.MessageEntry, 0, n)

	for len(result) < n && pq.totalCount > 0 {
		// Check from highest priority (0) to lowest (3)
		for i := 0; i < shared.MessagePriorityCount && len(result) < n; i++ {
			for pq.queues[i].Len() > 0 && len(result) < n {
				entry, ok := pq.queues[i].PopFront()
				if ok {
					pq.totalCount--
					result = append(result, entry)
				}
			}
		}
	}

	return result
}

// RemoveLowestPriority removes and returns the lowest priority message. O(1).
// Used for overflow handling - removes from Low first, then Normal.
func (pq *PriorityQueue) RemoveLowestPriority() *shared.MessageEntry {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// Check from lowest priority (3) to highest (0)
	// But only remove from Low and Normal to preserve important messages
	for _, priority := range []shared.MessagePriority{
		shared.MessagePriorityLow,
		shared.MessagePriorityNormal,
	} {
		if pq.queues[priority].Len() > 0 {
			entry, ok := pq.queues[priority].PopFront()
			if ok {
				pq.totalCount--
				return entry
			}
		}
	}

	// Fall back to any queue if no low/normal messages
	for i := shared.MessagePriorityCount - 1; i >= 0; i-- {
		if pq.queues[i].Len() > 0 {
			entry, ok := pq.queues[i].PopFront()
			if ok {
				pq.totalCount--
				return entry
			}
		}
	}

	return nil
}

// Peek returns the highest priority message without removing it. O(1).
func (pq *PriorityQueue) Peek() *shared.MessageEntry {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	for i := 0; i < shared.MessagePriorityCount; i++ {
		if pq.queues[i].Len() > 0 {
			entry, ok := pq.queues[i].PeekFront()
			if ok {
				return entry
			}
		}
	}
	return nil
}

// Clear removes all elements from all priority queues.
func (pq *PriorityQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for i := 0; i < shared.MessagePriorityCount; i++ {
		pq.queues[i].Clear()
	}
	pq.totalCount = 0
}

// Find finds a message entry matching the predicate. O(n).
func (pq *PriorityQueue) Find(predicate func(*shared.MessageEntry) bool) (*shared.MessageEntry, bool) {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	for i := 0; i < shared.MessagePriorityCount; i++ {
		entry, found := pq.queues[i].Find(predicate)
		if found {
			return entry, true
		}
	}
	return nil, false
}

// FindByMessageID finds a message by its ID. O(n).
func (pq *PriorityQueue) FindByMessageID(messageID string) (*shared.MessageEntry, bool) {
	return pq.Find(func(entry *shared.MessageEntry) bool {
		return entry.Message != nil && entry.Message.ID == messageID
	})
}

// LenByPriority returns the count for a specific priority level.
func (pq *PriorityQueue) LenByPriority(priority shared.MessagePriority) int {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	if priority < 0 || int(priority) >= shared.MessagePriorityCount {
		return 0
	}
	return pq.queues[priority].Len()
}

// Stats returns statistics about the queue.
func (pq *PriorityQueue) Stats() PriorityQueueStats {
	pq.mu.RLock()
	defer pq.mu.RUnlock()

	stats := PriorityQueueStats{
		Total:  pq.totalCount,
		Counts: make([]int, shared.MessagePriorityCount),
	}

	for i := 0; i < shared.MessagePriorityCount; i++ {
		stats.Counts[i] = pq.queues[i].Len()
	}

	return stats
}

// PriorityQueueStats holds statistics about a priority queue.
type PriorityQueueStats struct {
	Total  int   `json:"total"`
	Counts []int `json:"counts"` // Count per priority level
}
