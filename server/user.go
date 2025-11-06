package main

import (
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

type LRUCache struct {
	capacity int
	lock     sync.RWMutex
	cache    map[string]*cacheEntry
}

type cacheEntry struct {
	user       *model.User
	lastAccess time.Time
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*cacheEntry),
	}
}

func (c *LRUCache) Get(key string) (*model.User, bool) {
	// Fast path: RLock for lookup
	c.lock.RLock()
	entry, found := c.cache[key]
	c.lock.RUnlock()

	if !found {
		return &model.User{}, false
	}

	// Update access time (lock-free for reads, will use mutex for accuracy)
	// Using write lock briefly to update access time
	c.lock.Lock()
	defer c.lock.Unlock()
	entry.lastAccess = time.Now()

	return entry.user, true
}

func (c *LRUCache) Put(key string, user *model.User) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Update the value if it already exists
	if entry, found := c.cache[key]; found {
		entry.user = user
		entry.lastAccess = time.Now()
		return
	}

	// If the cache is at capacity, remove the least recently used item
	if len(c.cache) >= c.capacity {
		c.evictOldest()
	}

	// Add the new item to the cache
	c.cache[key] = &cacheEntry{
		user:       user,
		lastAccess: time.Now(),
	}
}

// evictOldest removes the least recently used item from the cache
// Must be called with lock held
func (c *LRUCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for k, v := range c.cache {
		if oldestKey == "" || v.lastAccess.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.lastAccess
		}
	}

	if oldestKey != "" {
		delete(c.cache, oldestKey)
	}
}

// Remove removes an entry from the cache
func (c *LRUCache) Remove(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	delete(c.cache, key)
}
