// Package messaging provides high-performance message bus implementation.
package messaging

import (
	"sync"
)

// Deque is a double-ended queue with O(1) push/pop operations using a circular buffer.
type Deque[T any] struct {
	buffer   []T
	head     int
	tail     int
	count    int
	capacity int
	mu       sync.RWMutex
}

// NewDeque creates a new Deque with the given initial capacity.
func NewDeque[T any](initialCapacity int) *Deque[T] {
	if initialCapacity < 1 {
		initialCapacity = 16
	}
	return &Deque[T]{
		buffer:   make([]T, initialCapacity),
		capacity: initialCapacity,
	}
}

// Len returns the number of elements in the deque.
func (d *Deque[T]) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.count
}

// IsEmpty returns true if the deque is empty.
func (d *Deque[T]) IsEmpty() bool {
	return d.Len() == 0
}

// grow doubles the capacity of the deque.
func (d *Deque[T]) grow() {
	newCapacity := d.capacity * 2
	newBuffer := make([]T, newCapacity)

	// Copy elements in order from head to tail
	for i := 0; i < d.count; i++ {
		newBuffer[i] = d.buffer[(d.head+i)%d.capacity]
	}

	d.buffer = newBuffer
	d.head = 0
	d.tail = d.count
	d.capacity = newCapacity
}

// PushBack adds an element to the back of the deque. O(1) amortized.
func (d *Deque[T]) PushBack(item T) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.count == d.capacity {
		d.grow()
	}

	d.buffer[d.tail] = item
	d.tail = (d.tail + 1) % d.capacity
	d.count++
}

// PushFront adds an element to the front of the deque. O(1) amortized.
func (d *Deque[T]) PushFront(item T) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.count == d.capacity {
		d.grow()
	}

	d.head = (d.head - 1 + d.capacity) % d.capacity
	d.buffer[d.head] = item
	d.count++
}

// PopFront removes and returns the element at the front of the deque. O(1).
func (d *Deque[T]) PopFront() (T, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var zero T
	if d.count == 0 {
		return zero, false
	}

	item := d.buffer[d.head]
	d.buffer[d.head] = zero // Help GC
	d.head = (d.head + 1) % d.capacity
	d.count--
	return item, true
}

// PopBack removes and returns the element at the back of the deque. O(1).
func (d *Deque[T]) PopBack() (T, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var zero T
	if d.count == 0 {
		return zero, false
	}

	d.tail = (d.tail - 1 + d.capacity) % d.capacity
	item := d.buffer[d.tail]
	d.buffer[d.tail] = zero // Help GC
	d.count--
	return item, true
}

// PeekFront returns the element at the front of the deque without removing it. O(1).
func (d *Deque[T]) PeekFront() (T, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var zero T
	if d.count == 0 {
		return zero, false
	}

	return d.buffer[d.head], true
}

// PeekBack returns the element at the back of the deque without removing it. O(1).
func (d *Deque[T]) PeekBack() (T, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var zero T
	if d.count == 0 {
		return zero, false
	}

	return d.buffer[(d.tail-1+d.capacity)%d.capacity], true
}

// Clear removes all elements from the deque.
func (d *Deque[T]) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()

	var zero T
	for i := 0; i < d.count; i++ {
		d.buffer[(d.head+i)%d.capacity] = zero
	}

	d.head = 0
	d.tail = 0
	d.count = 0
}

// Find returns the first element matching the predicate. O(n).
func (d *Deque[T]) Find(predicate func(T) bool) (T, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var zero T
	for i := 0; i < d.count; i++ {
		idx := (d.head + i) % d.capacity
		if predicate(d.buffer[idx]) {
			return d.buffer[idx], true
		}
	}
	return zero, false
}

// FindAndRemove finds and removes the first element matching the predicate. O(n).
func (d *Deque[T]) FindAndRemove(predicate func(T) bool) (T, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var zero T
	for i := 0; i < d.count; i++ {
		idx := (d.head + i) % d.capacity
		if predicate(d.buffer[idx]) {
			item := d.buffer[idx]

			// Shift remaining elements (O(n) but acceptable for rare operations)
			for j := i; j < d.count-1; j++ {
				currentIdx := (d.head + j) % d.capacity
				nextIdx := (d.head + j + 1) % d.capacity
				d.buffer[currentIdx] = d.buffer[nextIdx]
			}

			d.tail = (d.tail - 1 + d.capacity) % d.capacity
			d.buffer[d.tail] = zero
			d.count--
			return item, true
		}
	}
	return zero, false
}

// ForEach iterates over all elements in the deque.
func (d *Deque[T]) ForEach(fn func(T)) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	for i := 0; i < d.count; i++ {
		idx := (d.head + i) % d.capacity
		fn(d.buffer[idx])
	}
}

// ToSlice returns all elements as a slice.
func (d *Deque[T]) ToSlice() []T {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]T, d.count)
	for i := 0; i < d.count; i++ {
		result[i] = d.buffer[(d.head+i)%d.capacity]
	}
	return result
}

// At returns the element at the given index. O(1).
func (d *Deque[T]) At(index int) (T, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var zero T
	if index < 0 || index >= d.count {
		return zero, false
	}

	return d.buffer[(d.head+index)%d.capacity], true
}

// Capacity returns the current capacity of the deque.
func (d *Deque[T]) Capacity() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.capacity
}
