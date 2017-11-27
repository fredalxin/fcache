package fcache

import (
	"time"
	"sync"
	"fmt"
	"io"
	"encoding/gob"
	"os"
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

func (c *Cache) Update(k string, v interface{}, d time.Duration) error {
	c.mu.Lock()
	_, ok := c.get(k)
	if !ok {
		c.mu.Lock()
		return fmt.Errorf("Item %s doesn't exist", k)
	}
	c.set(k, v, d)
	c.mu.Unlock()
	return nil
}

func (c *Cache) Inc(k string, n int64) error {
	c.mu.Lock()
	_, ok := c.get(k)
	if !ok {
		c.mu.Lock()
		return fmt.Errorf("Item %s doesn't exist", k)
	}
	//c.set(k, v, d)
	c.mu.Unlock()
	return nil
}

func (c *Cache) Delete(k string) {
	c.mu.Lock()
	c.delete(k)
	c.mu.Unlock()
}

func (c *Cache) Save(w io.Writer) (err error) {
	enc := gob.NewEncoder(w)
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("Error registering item types with Gob library")
		}
	}()
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range c.items {
		gob.Register(v.Object)
	}
	err = enc.Encode(&c.items)
	return
}

func (c *Cache) SaveToFile(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	if err = c.Save(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (c *Cache) Load(r io.Reader) error {
	dec := gob.NewDecoder(r)
	items := map[string]Item{}
	err := dec.Decode(&items)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range items {
		item, ok := c.items[k]
		if !ok || item.Expired() {
			c.items[k] = v
		}
	}
	return err
}

func (c *Cache) LoadFromFile(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	if err = c.Load(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (c *Cache) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

func (c *Cache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = map[string]Item{}
}

func (c *Cache) StopGc() {
	c.stopGc <- true
}

func NewCache(defaultExpiration, gcInterval time.Duration) *Cache {
	c := &Cache{
		defaultExpiration: defaultExpiration,
		gcInterval:        gcInterval,
		items:             map[string]Item{},
		stopGc:            make(chan bool),
	}
	go c.gcLoop()
	return c
}
