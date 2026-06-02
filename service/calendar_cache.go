package service

import (
	"sync"
	"time"
	"tutorgo/models"
)

const calendarCacheTTL = 60 * time.Second

type calendarCacheEntry struct {
	data      []models.CalendarLesson
	expiresAt time.Time
}

// calendarCache is a per-tutor in-memory cache for /calendar responses.
// Invalidated whenever lessons or payments change for a given tutor.
type calendarCache struct {
	mu      sync.RWMutex
	entries map[string]map[string]*calendarCacheEntry // tutorID → cacheKey → entry
}

var globalCalendarCache = &calendarCache{
	entries: make(map[string]map[string]*calendarCacheEntry),
}

func init() {
	go globalCalendarCache.sweepExpired()
}

// sweepExpired runs every 5 minutes and removes entries whose TTL has elapsed.
// Without this, stale entries accumulate in memory indefinitely.
func (c *calendarCache) sweepExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for tutorID, byRange := range c.entries {
			for key, entry := range byRange {
				if now.After(entry.expiresAt) {
					delete(byRange, key)
				}
			}
			if len(byRange) == 0 {
				delete(c.entries, tutorID)
			}
		}
		c.mu.Unlock()
	}
}

func (c *calendarCache) get(tutorID, from, to string) ([]models.CalendarLesson, bool) {
	key := from + "|" + to
	c.mu.RLock()
	defer c.mu.RUnlock()
	byTutor, ok := c.entries[tutorID]
	if !ok {
		return nil, false
	}
	entry, ok := byTutor[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func (c *calendarCache) set(tutorID, from, to string, data []models.CalendarLesson) {
	key := from + "|" + to
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries[tutorID] == nil {
		c.entries[tutorID] = make(map[string]*calendarCacheEntry)
	}
	c.entries[tutorID][key] = &calendarCacheEntry{
		data:      data,
		expiresAt: time.Now().Add(calendarCacheTTL),
	}
}

// Invalidate drops all cached ranges for a tutor — call on any lesson or payment mutation.
func (c *calendarCache) Invalidate(tutorID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, tutorID)
}
