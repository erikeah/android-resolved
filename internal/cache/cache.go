package cache

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

type entry struct {
	msg       *dns.Msg
	expiresAt time.Time
}

type Cache struct {
	mu     sync.RWMutex
	store  map[string]*entry
	size   int
	minTTL time.Duration
	maxTTL time.Duration
}

func New(size int, minTTL, maxTTL time.Duration) *Cache {
	return &Cache{
		store:  make(map[string]*entry, size),
		size:   size,
		minTTL: minTTL,
		maxTTL: maxTTL,
	}
}

func key(name string, qtype uint16) string {
	return name + "\x00" + string(rune(qtype))
}

func clampTTL(ttl uint32, min, max time.Duration) time.Duration {
	d := time.Duration(ttl) * time.Second
	if d < min {
		d = min
	}
	if max > 0 && d > max {
		d = max
	}
	return d
}

func (c *Cache) Get(name string, qtype uint16) *dns.Msg {
	c.mu.RLock()
	e, ok := c.store[key(name, qtype)]
	c.mu.RUnlock()

	if !ok {
		return nil
	}

	if time.Now().After(e.expiresAt) {
		c.mu.Lock()
		delete(c.store, key(name, qtype))
		c.mu.Unlock()
		return nil
	}

	return e.msg.Copy()
}

func (c *Cache) Set(name string, qtype uint16, msg *dns.Msg) {
	if msg == nil || len(msg.Answer) == 0 {
		return
	}

	var ttl uint32
	for _, ans := range msg.Answer {
		if ans.Header().Ttl > ttl {
			ttl = ans.Header().Ttl
		}
	}
	if ttl == 0 {
		return
	}

	expires := clampTTL(ttl, c.minTTL, c.maxTTL)

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.store) >= c.size {
		for k := range c.store {
			delete(c.store, k)
			break
		}
	}

	c.store[key(name, qtype)] = &entry{
		msg:       msg.Copy(),
		expiresAt: time.Now().Add(expires),
	}
}

func (c *Cache) Clear() {
	c.mu.Lock()
	c.store = make(map[string]*entry, c.size)
	c.mu.Unlock()
}

func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
}
