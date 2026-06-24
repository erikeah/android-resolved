package resolver

import (
	"fmt"
	"time"

	"github.com/anomalyco/android-resolved/internal/router"
)

func NewFromMatch(match *router.Match, timeout time.Duration) (Resolver, error) {
	switch match.Rule.Protocol {
	case "udp":
		return NewDirectResolver(match.Upstream, false, timeout), nil
	case "tcp":
		return NewDirectResolver(match.Upstream, true, timeout), nil
	case "mdns":
		return NewMDNSResolver(timeout), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", match.Rule.Protocol)
	}
}
