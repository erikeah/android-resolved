package resolver

import (
	"fmt"
	"time"

	"github.com/miekg/dns"
)

type MDNSResolver struct {
	addr   string
	client dns.Client
}

func NewMDNSResolver(timeout time.Duration) *MDNSResolver {
	return &MDNSResolver{
		addr: "224.0.0.251:5353",
		client: dns.Client{
			Net:          "udp",
			Timeout:      timeout,
			UDPSize:      65535,
			ReadTimeout:  timeout,
			WriteTimeout: timeout,
		},
	}
}

func (r *MDNSResolver) Exchange(msg *dns.Msg) (*dns.Msg, error) {
	reply, _, err := r.client.Exchange(msg, r.addr)
	if err != nil {
		return nil, fmt.Errorf("mdns exchange: %w", err)
	}
	return reply, nil
}

func (r *MDNSResolver) String() string {
	return "mdns(multicast)"
}
