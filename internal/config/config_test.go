package config

import (
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config should be valid: %v", err)
	}
}

func TestValidateMissingDomain(t *testing.T) {
	cfg := &Config{
		Rules: []Rule{
			{Domain: "", Upstream: "1.1.1.1:53", Protocol: "udp"},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty domain")
	}
}

func TestValidateMissingUpstream(t *testing.T) {
	cfg := &Config{
		Rules: []Rule{
			{Domain: ".", Upstream: "", Protocol: "udp"},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty upstream")
	}
}

func TestValidateMDNSNoUpstream(t *testing.T) {
	cfg := &Config{
		Rules: []Rule{
			{Domain: ".local", Upstream: "", Protocol: "mdns"},
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("mdns should not require upstream: %v", err)
	}
}
