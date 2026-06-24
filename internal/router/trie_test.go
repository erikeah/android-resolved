package router

import (
	"testing"

	"github.com/anomalyco/android-resolved/internal/config"
)

func TestTrieRouterExactMatch(t *testing.T) {
	rules := []config.Rule{
		{Domain: "google.com", Upstream: "8.8.8.8:53", Protocol: "udp"},
		{Domain: ".", Upstream: "1.1.1.1:53", Protocol: "udp"},
	}
	tr := NewTrieRouter(rules)

	m := tr.Route("google.com")
	if m == nil || m.Upstream != "8.8.8.8:53" {
		t.Fatalf("expected 8.8.8.8:53, got %v", m)
	}
}

func TestTrieRouterSubdomainMatch(t *testing.T) {
	rules := []config.Rule{
		{Domain: "corp.example.com", Upstream: "10.0.0.1:53", Protocol: "udp"},
		{Domain: ".", Upstream: "1.1.1.1:53", Protocol: "udp"},
	}
	tr := NewTrieRouter(rules)

	m := tr.Route("api.corp.example.com")
	if m == nil || m.Upstream != "10.0.0.1:53" {
		t.Fatalf("expected 10.0.0.1:53, got %v", m)
	}
}

func TestTrieRouterCatchAll(t *testing.T) {
	rules := []config.Rule{
		{Domain: "google.com", Upstream: "8.8.8.8:53", Protocol: "udp"},
		{Domain: ".", Upstream: "1.1.1.1:53", Protocol: "udp"},
	}
	tr := NewTrieRouter(rules)

	m := tr.Route("example.com")
	if m == nil || m.Upstream != "1.1.1.1:53" {
		t.Fatalf("expected 1.1.1.1:53, got %v", m)
	}
}

func TestTrieRouterNoMatch(t *testing.T) {
	tr := NewTrieRouter(nil)

	m := tr.Route("example.com")
	if m != nil {
		t.Fatalf("expected nil, got %v", m)
	}
}

func TestTrieRouterLongestSuffix(t *testing.T) {
	rules := []config.Rule{
		{Domain: "example.com", Upstream: "10.0.0.1:53", Protocol: "udp"},
		{Domain: "sub.example.com", Upstream: "10.0.0.2:53", Protocol: "udp"},
		{Domain: ".", Upstream: "1.1.1.1:53", Protocol: "udp"},
	}
	tr := NewTrieRouter(rules)

	m := tr.Route("deep.sub.example.com")
	if m == nil || m.Upstream != "10.0.0.2:53" {
		t.Fatalf("expected 10.0.0.2:53 for longest suffix, got %v", m)
	}
}

func TestTrieRouterUpdate(t *testing.T) {
	tr := NewTrieRouter([]config.Rule{
		{Domain: "old.com", Upstream: "10.0.0.1:53", Protocol: "udp"},
	})

	tr.Update([]config.Rule{
		{Domain: "new.com", Upstream: "10.0.0.2:53", Protocol: "udp"},
	})

	m := tr.Route("old.com")
	if m != nil {
		t.Fatalf("expected old rule to be gone")
	}

	m = tr.Route("new.com")
	if m == nil || m.Upstream != "10.0.0.2:53" {
		t.Fatalf("expected new rule, got %v", m)
	}
}
