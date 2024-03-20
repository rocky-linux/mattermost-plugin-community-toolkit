package main

import (
	"container/list"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
)

type LRUCache struct {
	capacity int
	lock     sync.Mutex
	cache    map[string]*list.Element
	lruList  *list.List // List to maintain LRU order
}

type entry struct {
	key  string
	user *model.User
}

func NewLRUCache(capacity int) *LRUCache {
	return &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		lruList:  list.New(),
	}
}

func (c *LRUCache) Get(key string) (*model.User, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if elem, found := c.cache[key]; found {
		c.lruList.MoveToFront(elem) // Mark as most recently used
		return elem.Value.(entry).user, true
	}
	return &model.User{}, false // Return zero value if not found
}

func (c *LRUCache) Put(key string, user *model.User) {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Update the value if it already exists and move it to the front
	if elem, found := c.cache[key]; found {
		c.lruList.MoveToFront(elem)
		elem.Value = entry{key, user}
		return
	}

	// If the cache is at capacity, remove the least recently used item
	if c.lruList.Len() == c.capacity {
		oldest := c.lruList.Back()
		if oldest != nil {
			c.lruList.Remove(oldest)
			delete(c.cache, oldest.Value.(entry).key)
		}
	}

	// Add the new item to the cache and LRU list
	elem := c.lruList.PushFront(entry{key, user})
	c.cache[key] = elem
}
