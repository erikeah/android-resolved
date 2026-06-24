package config

import (
	"fmt"
	"time"
)

type Protocol string

const (
	ProtocolUDP  Protocol = "udp"
	ProtocolTCP  Protocol = "tcp"
	ProtocolMDNS Protocol = "mdns"
)

type Rule struct {
	Domain   string   `json:"domain"`
	Upstream string   `json:"upstream"`
	Protocol Protocol `json:"protocol"`
}

type CacheConfig struct {
	Size   int           `json:"size"`
	MinTTL time.Duration `json:"min_ttl"`
	MaxTTL time.Duration `json:"max_ttl"`
}

type Config struct {
	Rules []Rule      `json:"rules"`
	Cache CacheConfig `json:"cache"`
}

func Default() *Config {
	return &Config{
		Rules: []Rule{
			{Domain: ".local", Protocol: ProtocolMDNS},
			{Domain: ".", Upstream: "1.1.1.1", Protocol: ProtocolUDP},
		},
		Cache: CacheConfig{
			Size:   4096,
			MinTTL: 60,
			MaxTTL: 3600,
		},
	}
}

func (c *Config) Validate() error {
	for i, r := range c.Rules {
		if r.Domain == "" {
			return fmt.Errorf("rule %d: domain is required", i)
		}
		switch r.Protocol {
		case ProtocolUDP, ProtocolTCP, ProtocolMDNS:
		case "":
			return fmt.Errorf("rule %d: protocol is required", i)
		default:
			return fmt.Errorf("rule %d: unknown protocol %q", i, r.Protocol)
		}
		if r.Protocol != ProtocolMDNS && r.Upstream == "" {
			return fmt.Errorf("rule %d: upstream is required for protocol %s", i, r.Protocol)
		}
	}
	return nil
}
