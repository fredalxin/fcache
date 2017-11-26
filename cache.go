package fcache

import (
	"time"
	"sync"
	"fmt"
)

const (
	// 没有过期时间标志
	NoExpiration time.Duration = -1
	// 默认的过期时间
	DefaultExpiration time.Duration = 0
)

type Cache struct {
	defaultExpiration time.Duration
	items             map[string]Item
	mu                sync.RWMutex
	gcInterval        time.Duration
	stopGc            chan bool
}

func (c *Cache) gcLoop() {
	ticker := time.NewTicker(c.gcInterval)
	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-c.stopGc:
			ticker.Stop()
			return
		}
	}
}
func (c *Cache) DeleteExpired() {
	now := time.Now().UnixNano()
	c.mu.Lock()
	defer c.mu.Unlock()

	for k, v := range c.items {
		if v.Expiration > 0 && now > v.Expiration {
			c.delete(k)
		}
	}
}

func (c *Cache) delete(k string) {
	delete(c.items, k)
}

func (c *Cache) set(k string, v interface{}, d time.Duration) {
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.items[k] = Item{
		Object:     v,
		Expiration: e,
	}
}

func (c *Cache) Set(k string, v interface{}, d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.set(k, v, d)
}

func (c *Cache) Add(k string, v interface{}, d time.Duration) error {
	c.mu.Lock()
	_, ok := c.get(k)
	if ok {
		c.mu.Unlock()
		return fmt.Errorf("Item % s already exists", k)
	}
	c.set(k, v, d)
	c.mu.Unlock()
	return nil
}

func (c *Cache) get(k string) (interface{}, bool) {
	item, ok := c.items[k]
	if !ok {
		return nil, false
	}
	if item.Expired() {
		return nil, false
	}
	return item.Object, true
}

func (c *Cache) Get(k string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.get(k)
}
