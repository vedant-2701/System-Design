// lrucache.go
package lrucache

import (
	"fmt"
	"sync"
)

// node is an internal doubly linked list node.
// Not exported — implementation detail only.
type node[K comparable, V any] struct {
	key   K
	value V
	prev  *node[K, V]
	next  *node[K, V]
}

// LRUCache is a thread-safe, generic LRU cache.
// Zero value is not usable — must be created via New().
type LRUCache[K comparable, V any] struct {
	capacity int
	length   int
	cache    map[K]*node[K, V]
	head     *node[K, V] // sentinel MRU end
	tail     *node[K, V] // sentinel LRU end
	mu       sync.Mutex
}

// New creates a new LRUCache with the given capacity.
// Returns an error if capacity is not positive.
func New[K comparable, V any](capacity int) (*LRUCache[K, V], error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("cache capacity must be positive, got %d", capacity)
	}

	head := &node[K, V]{}
	tail := &node[K, V]{}
	head.next = tail
	tail.prev = head

	return &LRUCache[K, V]{
		capacity: capacity,
		cache:    make(map[K]*node[K, V], capacity),
		head:     head,
		tail:     tail,
	}, nil
}

// Get retrieves a value by key. Returns the value and true if found,
// zero value and false if not. Marks the entry as most recently used.
func (c *LRUCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, exists := c.cache[key]
	if !exists {
		var zero V
		return zero, false
	}
	c.moveToHead(n)
	return n.value, true
}

// Put inserts or updates a key-value pair.
// If at capacity, the least recently used entry is evicted.
func (c *LRUCache[K, V]) Put(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, exists := c.cache[key]; exists {
		existing.value = value
		c.moveToHead(existing)
		return
	}

	newNode := &node[K, V]{key: key, value: value}
	c.cache[key] = newNode
	c.insertAtHead(newNode)
	c.length++

	if c.length > c.capacity {
		lru := c.tail.prev
		c.removeNode(lru)
		delete(c.cache, lru.key)
		c.length--
	}
}

// Size returns the current number of entries in the cache.
func (c *LRUCache[K, V]) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.length
}

// Clear removes all entries from the cache.
func (c *LRUCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[K]*node[K, V], c.capacity)
	c.head.next = c.tail
	c.tail.prev = c.head
	c.length = 0
}

// --- Private list operations ---
// Must only be called while holding mu.

func (c *LRUCache[K, V]) insertAtHead(n *node[K, V]) {
	n.prev = c.head
	n.next = c.head.next
	c.head.next.prev = n
	c.head.next = n
}

func (c *LRUCache[K, V]) removeNode(n *node[K, V]) {
	n.prev.next = n.next
	n.next.prev = n.prev
}

func (c *LRUCache[K, V]) moveToHead(n *node[K, V]) {
	c.removeNode(n)
	c.insertAtHead(n)
}
