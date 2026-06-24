package cache

import (
	"testing"
	"time"

	"github.com/miekg/dns"
)

func a(name, ip string) *dns.Msg {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(name), dns.TypeA)
	rr, _ := dns.NewRR(dns.Fqdn(name) + " 300 IN A " + ip)
	msg.Answer = append(msg.Answer, rr)
	return msg
}

func TestCacheSetGet(t *testing.T) {
	c := New(100, 0, 0)

	c.Set("example.com", dns.TypeA, a("example.com", "1.2.3.4"))

	got := c.Get("example.com", dns.TypeA)
	if got == nil {
		t.Fatal("expected cached result")
	}
	if len(got.Answer) == 0 {
		t.Fatal("expected answer")
	}
}

func TestCacheMiss(t *testing.T) {
	c := New(100, 0, 0)

	got := c.Get("example.com", dns.TypeA)
	if got != nil {
		t.Fatal("expected nil for uncached query")
	}
}

func TestCacheExpiry(t *testing.T) {
	c := New(100, 0, time.Nanosecond)

	msg := a("example.com", "1.2.3.4")
	msg.Answer[0].Header().Ttl = 0

	c.Set("example.com", dns.TypeA, msg)

	time.Sleep(time.Millisecond)

	got := c.Get("example.com", dns.TypeA)
	if got != nil {
		t.Fatal("expected expired entry")
	}
}

func TestCacheEviction(t *testing.T) {
	c := New(2, 0, 0)

	c.Set("a.com", dns.TypeA, a("a.com", "1.1.1.1"))
	c.Set("b.com", dns.TypeA, a("b.com", "2.2.2.2"))
	c.Set("c.com", dns.TypeA, a("c.com", "3.3.3.3"))

	if c.Len() > 2 {
		t.Fatalf("cache size exceeded: %d", c.Len())
	}
}

func TestCacheClear(t *testing.T) {
	c := New(100, 0, 0)

	c.Set("a.com", dns.TypeA, a("a.com", "1.1.1.1"))
	c.Set("b.com", dns.TypeA, a("b.com", "2.2.2.2"))

	c.Clear()

	if c.Len() != 0 {
		t.Fatalf("expected empty cache, got %d", c.Len())
	}
}
