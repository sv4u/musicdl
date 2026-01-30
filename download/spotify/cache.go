package spotify

import (
	"container/list"
	"sync"
	"sync/atomic"
	"time"
)

// CacheStats holds cache statistics.
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int
	MaxSize   int
	HitRate   float64
}

// cacheEntry represents a single cache entry.
type cacheEntry struct {
	key       string
	value     interface{}
	expiresAt int64
	element   *list.Element // For LRU list
}

// TTLCache is a thread-safe TTL cache with LRU eviction.
type TTLCache struct {
	mu             sync.RWMutex
	cache          map[string]*cacheEntry
	lruList        *list.List // Doubly-linked list for LRU
	maxSize        int
	ttlSeconds     int
	hits           int64
	misses         int64
	evictions      int64
	stopCleanup    chan struct{}
	cleanupRunning bool
}

// NewTTLCache creates a new TTL cache.
func NewTTLCache(maxSize, ttlSeconds int) *TTLCache {
	return &TTLCache{
		cache:       make(map[string]*cacheEntry),
		lruList:     list.New(),
		maxSize:     maxSize,
		ttlSeconds:  ttlSeconds,
		stopCleanup: make(chan struct{}),
	}
}

// Get retrieves a value from the cache.
// Returns nil if not found or expired.
func (c *TTLCache) Get(key string) interface{} {
	c.mu.RLock()
	entry, exists := c.cache[key]
	c.mu.RUnlock()

	if !exists {
		atomic.AddInt64(&c.misses, 1)
		return nil
	}

	// Check expiration
	now := time.Now().Unix()
	if now >= entry.expiresAt {
		// Expired - remove it
		c.mu.Lock()
		// Double-check after acquiring write lock
		if e, stillExists := c.cache[key]; stillExists && e == entry {
			delete(c.cache, key)
			if entry.element != nil {
				c.lruList.Remove(entry.element)
			}
		}
		c.mu.Unlock()
		atomic.AddInt64(&c.misses, 1)
		return nil
	}

	// Move to front of LRU list (mark as recently used)
	c.mu.Lock()
	if entry.element != nil {
		c.lruList.MoveToFront(entry.element)
	}
	c.mu.Unlock()

	atomic.AddInt64(&c.hits, 1)
	return entry.value
}

// Set stores a value in the cache with TTL.
func (c *TTLCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().Unix()
	expiresAt := now + int64(c.ttlSeconds)

	// Check if key already exists
	if existing, exists := c.cache[key]; exists {
		// Update existing entry
		existing.value = value
		existing.expiresAt = expiresAt
		// Move to front
		if existing.element != nil {
			c.lruList.MoveToFront(existing.element)
		}
		return
	}

	// Check if we need to evict
	if len(c.cache) >= c.maxSize {
		// Evict LRU (tail of list)
		if c.lruList.Len() > 0 {
			back := c.lruList.Back()
			if back != nil {
				oldEntry := back.Value.(*cacheEntry)
				delete(c.cache, oldEntry.key)
				c.lruList.Remove(back)
				atomic.AddInt64(&c.evictions, 1)
			}
		}
	}

	// Create new entry
	entry := &cacheEntry{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}

	// Add to front of LRU list
	entry.element = c.lruList.PushFront(entry)
	c.cache[key] = entry
}

// Clear removes all entries from the cache.
func (c *TTLCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*cacheEntry)
	c.lruList = list.New()
}

// Stats returns cache statistics.
func (c *TTLCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	hits := atomic.LoadInt64(&c.hits)
	misses := atomic.LoadInt64(&c.misses)
	evictions := atomic.LoadInt64(&c.evictions)

	total := hits + misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return CacheStats{
		Hits:      hits,
		Misses:    misses,
		Evictions: evictions,
		Size:      len(c.cache),
		MaxSize:   c.maxSize,
		HitRate:   hitRate,
	}
}

// Size returns the current number of entries.
func (c *TTLCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.cache)
}

// StartCleanup starts a background goroutine to periodically clean up expired entries.
func (c *TTLCache) StartCleanup(interval time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cleanupRunning {
		return
	}

	c.cleanupRunning = true
	go c.cleanupLoop(interval)
}

// StopCleanup stops the background cleanup goroutine.
func (c *TTLCache) StopCleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.cleanupRunning {
		return
	}

	// Safely close channel - check if already closed
	select {
	case <-c.stopCleanup:
		// Already closed
	default:
		close(c.stopCleanup)
	}
	c.cleanupRunning = false
}

// cleanupLoop runs periodic cleanup of expired entries.
func (c *TTLCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanupExpired()
		case <-c.stopCleanup:
			return
		}
	}
}

// cleanupExpired removes expired entries from the cache.
func (c *TTLCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().Unix()
	toRemove := []*list.Element{}

	// Find expired entries
	for e := c.lruList.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*cacheEntry)
		if now >= entry.expiresAt {
			toRemove = append(toRemove, e)
		}
	}

	// Remove expired entries
	for _, e := range toRemove {
		entry := e.Value.(*cacheEntry)
		delete(c.cache, entry.key)
		c.lruList.Remove(e)
	}
}
