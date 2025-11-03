package main

import (
	"sync"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLRUCache(t *testing.T) {
	t.Run("creates cache with correct capacity", func(t *testing.T) {
		cache := NewLRUCache(50)
		require.NotNil(t, cache)
		assert.Equal(t, 50, cache.capacity)
		assert.NotNil(t, cache.cache)
		assert.NotNil(t, cache.lruList)
		assert.Equal(t, 0, cache.lruList.Len())
	})

	t.Run("creates cache with small capacity", func(t *testing.T) {
		cache := NewLRUCache(1)
		require.NotNil(t, cache)
		assert.Equal(t, 1, cache.capacity)
	})

	t.Run("creates cache with large capacity", func(t *testing.T) {
		cache := NewLRUCache(10000)
		require.NotNil(t, cache)
		assert.Equal(t, 10000, cache.capacity)
	})
}

func TestLRUCacheGet(t *testing.T) {
	t.Run("returns false for non-existent key", func(t *testing.T) {
		cache := NewLRUCache(5)
		user, found := cache.Get("non-existent")
		assert.False(t, found)
		assert.Equal(t, &model.User{}, user)
	})

	t.Run("returns cached user for existing key", func(t *testing.T) {
		cache := NewLRUCache(5)
		expectedUser := &model.User{
			Id:       "user-id-123",
			Username: "testuser",
			Email:    "test@example.com",
		}

		cache.Put(expectedUser.Id, expectedUser)

		user, found := cache.Get(expectedUser.Id)
		assert.True(t, found)
		assert.Equal(t, expectedUser.Id, user.Id)
		assert.Equal(t, expectedUser.Username, user.Username)
		assert.Equal(t, expectedUser.Email, user.Email)
	})

	t.Run("moves accessed item to front", func(t *testing.T) {
		cache := NewLRUCache(3)

		user1 := &model.User{Id: "1", Username: "user1"}
		user2 := &model.User{Id: "2", Username: "user2"}
		user3 := &model.User{Id: "3", Username: "user3"}

		cache.Put("1", user1)
		cache.Put("2", user2)
		cache.Put("3", user3)

		// cache should now reflect:
		// [3], [2], [1]

		// Access user1, moving it to front
		_, found := cache.Get("1")
		assert.True(t, found)

		// cache should now reflect:
		// [1], [3], [2]

		// Add a 4th user, which should evict user2 (least recently used)
		user4 := &model.User{Id: "4", Username: "user4"}
		cache.Put("4", user4)

		// cache should now reflect:
		// [4], [1], [3] --> [2] was evicted

		// User2 should be evicted
		_, found = cache.Get("2")
		assert.False(t, found)

		// User1 should still exist (recently accessed)
		_, found = cache.Get("1")
		assert.True(t, found)

		// User3 should still exist
		_, found = cache.Get("3")
		assert.True(t, found)

		// User4 should exist (just added)
		_, found = cache.Get("4")
		assert.True(t, found)
	})
}

func TestLRUCachePut(t *testing.T) {
	t.Run("adds new item to cache", func(t *testing.T) {
		cache := NewLRUCache(5)
		user := &model.User{
			Id:       "user-id",
			Username: "testuser",
		}

		cache.Put("user-id", user)

		cachedUser, found := cache.Get("user-id")
		assert.True(t, found)
		assert.Equal(t, user.Id, cachedUser.Id)
	})

	t.Run("updates existing item", func(t *testing.T) {
		cache := NewLRUCache(5)

		originalUser := &model.User{
			Id:       "user-id",
			Username: "original",
		}
		cache.Put(originalUser.Id, originalUser)

		updatedUser := &model.User{
			Id:       originalUser.Id,
			Username: "updated",
		}
		cache.Put(updatedUser.Id, updatedUser)

		cachedUser, found := cache.Get(originalUser.Id)
		assert.True(t, found)
		assert.Equal(t, updatedUser.Username, cachedUser.Username)
	})

	t.Run("evicts least recently used when at capacity", func(t *testing.T) {
		cache := NewLRUCache(3)

		user1 := &model.User{Id: "1"}
		user2 := &model.User{Id: "2"}
		user3 := &model.User{Id: "3"}
		user4 := &model.User{Id: "4"}

		cache.Put("1", user1)
		cache.Put("2", user2)
		cache.Put("3", user3)

		// Cache is at capacity, adding user4 should evict user1
		cache.Put("4", user4)

		_, found := cache.Get("1")
		assert.False(t, found, "user1 should have been evicted")

		_, found = cache.Get("2")
		assert.True(t, found, "user2 should still exist")

		_, found = cache.Get("3")
		assert.True(t, found, "user3 should still exist")

		_, found = cache.Get("4")
		assert.True(t, found, "user4 should exist")
	})

	t.Run("handles capacity of 1", func(t *testing.T) {
		cache := NewLRUCache(1)

		user1 := &model.User{Id: "1"}
		user2 := &model.User{Id: "2"}

		cache.Put("1", user1)
		cache.Put("2", user2)

		_, found := cache.Get("1")
		assert.False(t, found, "user1 should have been evicted")

		_, found = cache.Get("2")
		assert.True(t, found, "user2 should exist")
	})
}

func TestLRUCacheConcurrency(t *testing.T) {
	t.Run("handles concurrent reads safely", func(t *testing.T) {
		cache := NewLRUCache(100)

		// Pre-populate cache
		for i := 0; i < 50; i++ {
			user := &model.User{Id: string(rune(i)), Username: string(rune(i))}
			cache.Put(string(rune(i)), user)
		}

		// Concurrent reads
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				key := string(rune(index % 50))
				_, _ = cache.Get(key)
			}(i)
		}
		wg.Wait()
	})

	t.Run("handles concurrent writes safely", func(t *testing.T) {
		cache := NewLRUCache(100)

		// Concurrent writes
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				user := &model.User{Id: string(rune(index)), Username: string(rune(index))}
				cache.Put(string(rune(index)), user)
			}(i)
		}
		wg.Wait()

		// Verify cache state is consistent
		assert.Equal(t, cache.lruList.Len(), 100)
		assert.Equal(t, cache.lruList.Len(), len(cache.cache))
	})

	t.Run("handles mixed concurrent operations", func(t *testing.T) {
		cache := NewLRUCache(50)

		// Pre-populate some entries
		for i := 0; i < 25; i++ {
			user := &model.User{Id: string(rune(i))}
			cache.Put(string(rune(i)), user)
		}

		// Mixed read/write operations
		var wg sync.WaitGroup

		// Writers
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				user := &model.User{Id: string(rune(index + 100))}
				cache.Put(string(rune(index+100)), user)
			}(i)
		}

		// Readers
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				cache.Get(string(rune(index)))
			}(i)
		}

		wg.Wait()

		// Verify cache state is consistent
		assert.Equal(t, cache.lruList.Len(), 50)
		assert.Equal(t, cache.lruList.Len(), len(cache.cache))
	})
}

func TestLRUCacheEvictionOrder(t *testing.T) {
	t.Run("evicts in LRU order", func(t *testing.T) {
		cache := NewLRUCache(4)

		// Add users in order
		users := []*model.User{
			{Id: "1", Username: "user1"},
			{Id: "2", Username: "user2"},
			{Id: "3", Username: "user3"},
			{Id: "4", Username: "user4"},
		}

		for _, user := range users {
			cache.Put(user.Id, user)
		}

		// Access users to change their order: 2, 4, 1, 3 (from most to least recent)
		cache.Get("2")
		cache.Get("4")
		cache.Get("1")
		cache.Get("3")

		// Add a new user, should evict user2 (least recently used)
		newUser := &model.User{Id: "5", Username: "user5"}
		cache.Put("5", newUser)

		// Check that user2 was evicted
		_, found := cache.Get("2")
		assert.False(t, found, "user2 should have been evicted")

		// All others should still be present
		for _, id := range []string{"3", "4", "1", "5"} {
			_, found := cache.Get(id)
			assert.True(t, found, "user %s should still be in cache", id)
		}
	})
}

func TestLRUCacheMemoryBehavior(t *testing.T) {
	t.Run("does not exceed capacity", func(t *testing.T) {
		capacity := 100
		cache := NewLRUCache(capacity)

		// Add more items than capacity
		for i := 0; i < capacity*2; i++ {
			user := &model.User{Id: string(rune(i))}
			cache.Put(string(rune(i)), user)
		}

		// Verify cache doesn't exceed capacity
		assert.Equal(t, capacity, cache.lruList.Len())
		assert.Equal(t, capacity, len(cache.cache))
	})
}
